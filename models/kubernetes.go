package models

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego"
	v1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
)

var kubernetesClient *kubernetes.Clientset

func InitKubernetesClient() {
	// default kubeconfig path
	if KubeConfigPath == "" {
		KubeConfigPath = resolveKubeConfigPath()
	}
	kubernetesClient = initKubernetesClient()
	beego.Info("Kubernetes client initialized.")
}

func initKubernetesClient() *kubernetes.Clientset {
	kubeConfigPath := os.Getenv("KUBECONFIG")

	if kubeConfigPath == "" {
		if home := os.Getenv("HOME"); home != "" {
			kubeConfigPath = filepath.Join(home, ".kube", "config")
		} else if u, err := user.Current(); err == nil && u.HomeDir != "" {
			kubeConfigPath = filepath.Join(u.HomeDir, ".kube", "config")
		} else {
			kubeConfigPath = "/home/djuybu/.kube/config"
		}
	}

	beego.Info(fmt.Sprintf("Trying kubeconfig at: %s", kubeConfigPath))

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		beego.Warn(fmt.Sprintf("Failed to load kubeconfig from file (%s): %s", kubeConfigPath, err.Error()))
		config, err = rest.InClusterConfig()
		if err != nil {
			beego.Error(fmt.Sprintf("Cannot build in-cluster config: %s", err.Error()))
			panic(fmt.Errorf("unable to load any Kubernetes config: %w", err))
		}
		beego.Info("Using in-cluster Kubernetes configuration.")
	} else {
		beego.Info("Using external kubeconfig file.")
	}

	var clientQPS float32 = float32(len(Clouds) * len(Clouds) * 2)
	config.QPS = clientQPS
	config.Burst = int(clientQPS) * 2

	beego.Info(fmt.Sprintf("Because multi-cloud manager manages %d clouds, we set the Kubernetes clientset QPS as %g and Burst as %d", len(Clouds), config.QPS, config.Burst))

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		beego.Error(fmt.Sprintf("Create kubernetes client error: %s", err.Error()))
		panic(err)
	}

	return client
}

func resolveKubeConfigPath() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		return v
	}
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".kube", "config")
	}
	if h := os.Getenv("HOME"); h != "" {
		return filepath.Join(h, ".kube", "config")
	}
	return "/root/.kube/config"
}

func ListDeployment(namespace string) ([]v1.Deployment, error) {
	ctx := context.Background()
	deployments, err := kubernetesClient.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		beego.Error(fmt.Sprintf("List deployments error: %s", err.Error()))
		return []v1.Deployment{}, err
	}
	return deployments.Items, nil
}

func GetDeployment(namespace, name string) (*v1.Deployment, error) {
	ctx := context.Background()
	deployment, err := kubernetesClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Deployment %s/%s not found: %s", namespace, name, err.Error()))
		return nil, nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Get deployment %s/%s error: %s", namespace, name, err.Error()))
		return nil, err
	}
	return deployment, nil
}

func CreateDeployment(d *v1.Deployment) (*v1.Deployment, error) {
	ctx := context.Background()
	createdDeployment, err := kubernetesClient.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
	if err != nil {
		beego.Error(fmt.Sprintf("Create deployment %s/%s error: %s", d.Namespace, d.Name, err.Error()))
	}
	return createdDeployment, err
}

func DeleteDeployment(namespace, name string) error {
	ctx := context.Background()
	//deletePolicy := metav1.DeletePropagationForeground
	err := kubernetesClient.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{
		//PropagationPolicy: &deletePolicy,
	})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Deployment %s/%s not found: %s, do nothing", namespace, name, err.Error()))
		return nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Delete deployment %s/%s error: %s", namespace, name, err.Error()))
		return err
	}
	return nil
}

func WaitForDeployDeleted(timeout int, checkInterval int, deploy *v1.Deployment) error {
	return MyWaitFor(timeout, checkInterval, func() (bool, error) {
		if deploy == nil {
			beego.Info("Deployment does not exist.")
			return true, nil
		}
		pods := getAllPods(*deploy)
		beego.Info(fmt.Sprintf("Deployment [%s/%s] still has [%d] pods.", deploy.Namespace, deploy.Name, len(pods)))
		if len(pods) == 0 {
			return true, nil
		}
		return false, nil
	})
}

func CreateService(s *apiv1.Service) (*apiv1.Service, error) {
	ctx := context.Background()
	createdService, err := kubernetesClient.CoreV1().Services(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
	if err != nil {
		beego.Error(fmt.Sprintf("Create service %s/%s error: %s", s.Namespace, s.Name, err.Error()))
	}
	return createdService, err
}

func DeleteService(namespace, name string) error {
	ctx := context.Background()
	//deletePolicy := metav1.DeletePropagationForeground
	err := kubernetesClient.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{
		//PropagationPolicy: &deletePolicy,
	})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Service %s/%s not found: %s, do nothing", namespace, name, err.Error()))
		return nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Delete service %s/%s error: %s", namespace, name, err.Error()))
		return err
	}
	return nil
}

func GetService(namespace, name string) (*apiv1.Service, error) {
	ctx := context.Background()
	service, err := kubernetesClient.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Service %s/%s not found: %s", namespace, name, err.Error()))
		return nil, nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Get service %s/%s error: %s", namespace, name, err.Error()))
		return nil, err
	}
	return service, nil
}

func GetJob(namespace, name string) (*batchv1.Job, error) {
	ctx := context.Background()
	job, err := kubernetesClient.BatchV1().Jobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Job %s/%s not found: %s", namespace, name, err.Error()))
		return nil, nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Get job %s/%s error: %s", namespace, name, err.Error()))
		return nil, err
	}
	return job, nil
}

func CreateJob(j *batchv1.Job) (*batchv1.Job, error) {
	ctx := context.Background()
	createdJob, err := kubernetesClient.BatchV1().Jobs(j.Namespace).Create(ctx, j, metav1.CreateOptions{})
	if err != nil {
		beego.Error(fmt.Sprintf("Create job %s/%s error: %s", j.Namespace, j.Name, err.Error()))
	}
	return createdJob, err
}

func WaitForJobCompleted(timeout int, checkInterval int, jobNamespace string, jobName string) error {
	return MyWaitFor(timeout, checkInterval, func() (bool, error) {
		job, err := GetJob(jobNamespace, jobName)
		if err != nil {
			outErr := fmt.Errorf("Get Job [%s/%s], error: %w", jobNamespace, jobName, err)
			beego.Error(outErr)
			return false, nil
		}

		beego.Info(fmt.Sprintf("Job [%s/%s], job.Status.Active is [%d], job.Status.Failed is [%d], job.Status.Succeeded is [%d].", job.Namespace, job.Name, job.Status.Active, job.Status.Failed, job.Status.Succeeded))

		if job.Status.Active > 0 {
			beego.Info(fmt.Sprintf("Job [%s/%s] is running.", jobNamespace, jobName))
			return false, nil
		} else if job.Status.Failed > 0 {
			outErr := fmt.Errorf("Job [%s/%s] has failed, error: %w", jobNamespace, jobName, err)
			beego.Error(outErr)
			return false, outErr
		} else if job.Status.Succeeded > 0 {
			beego.Info(fmt.Sprintf("Job [%s/%s] is completed.", jobNamespace, jobName))
			return true, nil
		} else {
			beego.Info(fmt.Sprintf("Job [%s/%s] has not started yet.", jobNamespace, jobName))
			return false, nil
		}
	})
}

func DeleteJob(namespace, name string) error {
	ctx := context.Background()
	deletePolicy := metav1.DeletePropagationForeground
	err := kubernetesClient.BatchV1().Jobs(namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil && errors.IsNotFound(err) {
		beego.Info(fmt.Sprintf("Job %s/%s not found: %s, do nothing", namespace, name, err.Error()))
		return nil
	}
	if err != nil {
		beego.Error(fmt.Sprintf("Delete job %s/%s error: %s", namespace, name, err.Error()))
		return err
	}
	return nil
}

func ListPods(namespace string, listOptions metav1.ListOptions) ([]apiv1.Pod, error) {
	ctx := context.Background()
	pods, err := kubernetesClient.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		beego.Error(fmt.Sprintf("List pods error: %s", err.Error()))
		return []apiv1.Pod{}, err
	}
	return pods.Items, nil
}

// list all pods on a node in a namespace
func ListPodsOnNode(namespace string, nodeName string) ([]apiv1.Pod, error) {
	ctx := context.Background()
	pods, err := kubernetesClient.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName),
		},
	)
	if err != nil {
		ourErr := fmt.Errorf("List pods on node [%s] error: %w", nodeName, err)
		beego.Error(ourErr)
		return []apiv1.Pod{}, ourErr
	}
	return pods.Items, nil
}

func ListNodes(listOptions metav1.ListOptions) ([]apiv1.Node, error) {
	ctx := context.Background()
	nodes, err := kubernetesClient.CoreV1().Nodes().List(ctx, listOptions)
	if err != nil {
		beego.Error(fmt.Sprintf("List nodes error: %s", err.Error()))
		return []apiv1.Node{}, err
	}
	return nodes.Items, nil
}

func ListNodesNamePrefix(prefix string) ([]apiv1.Node, error) {
	// get all Kubernetes nodes
	allK8sNodes, err := ListNodes(metav1.ListOptions{})
	if err != nil {
		outErr := fmt.Errorf("cleanup auto-scheduling VMs, List Kubernetes Nodes Error: %w", err)
		beego.Error(outErr)
		return nil, outErr
	}

	// filter the nodes with the name prefix
	var outNodes []apiv1.Node
	for _, node := range allK8sNodes {
		if strings.HasPrefix(node.Name, prefix) {
			outNodes = append(outNodes, node)
		}
	}

	return outNodes, nil
}

func GetNode(name string, getOptions metav1.GetOptions) (*apiv1.Node, error) {
	ctx := context.Background()
	node, err := kubernetesClient.CoreV1().Nodes().Get(ctx, name, getOptions)
	if err != nil {
		beego.Error(fmt.Sprintf("Get node %s with options %v error: %s", name, getOptions, err.Error()))
		return nil, err
	}
	return node, nil
}

func DeleteNode(name string, deleteOptions metav1.DeleteOptions) error {
	ctx := context.Background()
	err := kubernetesClient.CoreV1().Nodes().Delete(ctx, name, deleteOptions)
	if err != nil {
		beego.Error(fmt.Sprintf("Delete node %s with options %v error: %s", name, deleteOptions, err.Error()))
		return err
	}
	return nil
}

func GetNodeInternalIp(node apiv1.Node) string {
	var ip string
	for _, addr := range node.Status.Addresses {
		// Kubernetes gets the VM IP from the network interface used as default gateway, and put it in the internal IP
		// pkg/kubelet/nodestatus/setters.go, func NodeAddress
		if addr.Type == apiv1.NodeInternalIP {
			ip = addr.Address
			break
		}
	}
	return ip
}

// Imitate kubectl get node xxx -o wide.
// pkg/printers/internalversion/printers.go, func printNode
func ExtractNodeStatus(node apiv1.Node) string {
	var status string = "Unknown"
	for _, cond := range node.Status.Conditions {
		if cond.Type == apiv1.NodeReady {
			if cond.Status == apiv1.ConditionTrue {
				status = string(cond.Type)
			} else {
				status = "Not" + string(cond.Type)
			}
		}
	}
	return status
}

// SSH to Kubernetes Master node to generate a kubeadm join command. The command will expire by default after 24 hours
// A Kubernetes cluster can have more than one tokens at a time.
func isLocalhost(ip string) bool {
	switch strings.TrimSpace(strings.ToLower(ip)) {
	case "127.0.0.1", "::1", "localhost":
		return true
	default:
		return false
	}
}

func runLocal(cmd string) ([]byte, error) {
	c := exec.Command("bash", "-lc", cmd)
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	return out.Bytes(), err
}

func GetJoinCmd() (string, error) {
	masterIP := beego.AppConfig.String("k8sMasterIP")
	sshKey := beego.AppConfig.String("k8sVmSshPrivateKey")
	sshUser := beego.AppConfig.DefaultString("k8sVmSshUser", "ubuntu")
	sshPort := SshPort

	// Luôn dùng sudo -n để không bị prompt password
	cmd := "sudo -n kubeadm token create --print-join-command"

	var out []byte
	var err error

	if isLocalhost(masterIP) {
		beego.Info("[GetJoinCmd] Running locally on localhost")
		out, err = runLocal(cmd)
	} else {
		beego.Info(fmt.Sprintf("[GetJoinCmd] SSH to %s:%d as %s", masterIP, sshPort, sshUser))
		client, derr := SshClientWithPem(sshKey, sshUser, masterIP, sshPort)
		if derr != nil {
			return "", fmt.Errorf("Create ssh client fail: %w", derr)
		}
		defer client.Close()
		out, err = SshOneCommand(client, cmd)
	}

	if err != nil {
		return "", fmt.Errorf("GetJoinCmd exec error: %w; output: %s", err, string(out))
	}

	join := strings.TrimSpace(string(out))
	if !strings.HasPrefix(join, "kubeadm join ") {
		return "", fmt.Errorf("unexpected join command output: %q", join)
	}

	beego.Info("[GetJoinCmd] got join command at ", time.Now().Format(time.RFC3339))
	return join, nil
}

// kubectl drain <nodeName> --ignore-daemonsets
func DrainNode(nodeName string) error {
	K8sMasterIP := beego.AppConfig.String("k8sMasterIP")
	sshPrivateKey := beego.AppConfig.String("k8sVmSshPrivateKey")
	sshPort := SshPort
	sshUser := SshRootUser

	sshClient, err := SshClientWithPem(sshPrivateKey, sshUser, K8sMasterIP, sshPort)
	if err != nil {
		outErr := fmt.Errorf("Create ssh client with SSH key fail: error: %w", err)
		beego.Error(outErr)
		return outErr
	}
	defer sshClient.Close()

	if _, err := SshOneCommand(sshClient, fmt.Sprintf("kubectl drain %s --ignore-daemonsets", nodeName)); err != nil {
		outErr := fmt.Errorf("ssh error: %w", err)
		beego.Error(outErr)
		return outErr
	}

	return nil
}

// Add or update a taint to a node
func TaintNode(nodeName string, taint *apiv1.Taint) error {
	// When we use the update api, Kubernetes will compare the resource version of the node in our request and that of the node in etcd. If they are different, there will be a conflict error.
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := GetNode(nodeName, metav1.GetOptions{})
		if err != nil {
			outErr := fmt.Errorf("Kubernetes get node: error: %w", err)
			beego.Error(outErr)
			return outErr
		}

		// immitate the code in "k8s.io/kubernetes/pkg/util/taints#AddOrUpdateTaint"
		nodeTaints := node.Spec.Taints
		var newTaints []apiv1.Taint
		updated := false
		for i := range nodeTaints {
			if taint.MatchTaint(&nodeTaints[i]) {
				if TaintEqual(taint, &nodeTaints[i]) {
					beego.Info(fmt.Sprintf("Node [%s] has already have the taint [%v]", nodeName, taint))
					return nil
				}
				// A taint with Key and Effect exist but value is not the same, so we need to update this taint
				newTaints = append(newTaints, *taint)
				updated = true
				continue
			}

			// this taint is different with what we need to add, we simply put it in the new taints
			newTaints = append(newTaints, nodeTaints[i])
		}

		// the node does not have a taint with the Key and Effect, so we need to create a new taint for this node
		if !updated {
			newTaints = append(newTaints, *taint)
		}

		node.Spec.Taints = newTaints

		ctx := context.Background()
		if _, err = kubernetesClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
			beego.Error(fmt.Sprintf("Update node [%s], error [%s]", nodeName, err))
			return err
		}

		return nil
	})

	if retryErr != nil {
		outErr := fmt.Errorf("Add taint [%s] to Node [%s], error [%w]", taint, nodeName, retryErr)
		beego.Error(outErr)
		return outErr
	}
	beego.Info(fmt.Sprintf("taint [%v] is added to Node [%s]", taint, nodeName))

	return nil
}

func TaintEqual(t1 *apiv1.Taint, t2 *apiv1.Taint) bool {
	return t1.Key == t2.Key && t1.Effect == t2.Effect && t1.Value == t2.Value
}

// Check whether a node has a taint
func NodeHasTaint(node *apiv1.Node, taint *apiv1.Taint) bool {
	for _, t := range node.Spec.Taints {
		if TaintEqual(&t, taint) {
			return true
		}
	}
	return false
}

// add a new node into Kubernetes cluster
func AddNode(vm IaasVm, joinCmd string) error {
	if len(vm.IPs) == 0 {
		outErr := fmt.Errorf("the input vm [%s] has no ip address", vm.Name)
		beego.Error(outErr)
		return outErr
	}

	// Prevent users from adding important VMs
	k8sMasterIP := beego.AppConfig.String("k8sMasterIP")
	dockerEngineIP := beego.AppConfig.String("dockerEngineIP")
	dockerRegistryIP := beego.AppConfig.String("dockerRegistryIP")

	if vm.IPs[0] == k8sMasterIP || vm.IPs[0] == dockerEngineIP || vm.IPs[0] == dockerRegistryIP {
		outErr := fmt.Errorf("the input vm [%s] is an important VM, so we refuse this risky request", vm.Name)
		beego.Error(outErr)
		return outErr
	}

	// get VM info
	name, ip := vm.Name, vm.IPs[0]
	sshPrivateKey := beego.AppConfig.String("k8sVmSshPrivateKey")
	sshPort := SshPort
	sshUser := beego.AppConfig.DefaultString("k8sVmSshUser", "ubuntu")

	// connect via SSH
	sshClient, err := SshClientWithPem(sshPrivateKey, sshUser, ip, sshPort)
	if err != nil {
		outErr := fmt.Errorf("Create ssh client with SSH key fail: error: %w", err)
		beego.Error(outErr)
		return outErr
	}
	defer sshClient.Close()

	// === FIX CONTAINERD CONFIG ===

	cmds := []string{
		"sudo mkdir -p /etc/containerd",
		"sudo test -f /etc/containerd/config.toml || sudo containerd config default | sudo tee /etc/containerd/config.toml",
		fmt.Sprintf("sudo sed -i 's/<IP>/%s/g' /etc/containerd/config.toml", k8sMasterIP),
		"sudo systemctl restart containerd",
		"sudo systemctl enable kubelet",
		fmt.Sprintf("sudo %s --node-name=%s", joinCmd, name),
	}

	for _, cmd := range cmds {
		out, err := SshOneCommand(sshClient, cmd)
		if err != nil {
			outErr := fmt.Errorf("ssh error at [%s]: %v, output: %s", cmd, err, string(out))
			beego.Error(outErr)
			return outErr
		}
	}

	// Wait for Kubernetes node to join
	if err := WaitForNodeJoin(WaitForTimeOut, 5, name); err != nil {
		outErr := fmt.Errorf("Wait for node %s join, error: %w", name, err)
		beego.Error(outErr)
		return outErr
	}

	return nil
}

func WaitForNodeJoin(timeout int, checkInterval int, nodeName string) error {
	return MyWaitFor(timeout, checkInterval, func() (bool, error) {
		node, err := GetNode(nodeName, metav1.GetOptions{})
		if err != nil {
			outErr := fmt.Errorf("Get Kubernetes node %s, error: %w", nodeName, err)
			beego.Error(outErr)
			return false, nil
		}
		beego.Info(fmt.Sprintf("node: [%v]", node))

		if ExtractNodeStatus(*node) == string(apiv1.NodeReady) {
			return true, nil
		}

		return false, nil
	})
}

// add some new nodes into Kubernetes cluster
func AddNodes(vms []IaasVm) []error {
	if len(vms) == 0 {
		beego.Info("no vms in the input")
		return nil
	}

	// use one joinCmd to add all nodes
	joinCmd, err := GetJoinCmd()
	if err != nil {
		outErr := fmt.Errorf("AddNodes, GetJoinCmd error: %w", err)
		beego.Error(outErr)
		return []error{outErr}
	}

	var errs []error
	// add all nodes in parallel
	// use one goroutine to add one node
	var wg sync.WaitGroup
	var errsMu sync.Mutex // the slice (errs) in golang is not safe for concurrent read/write
	for i := 0; i < len(vms); i++ {
		wg.Add(1)
		go func(vm IaasVm) {
			defer wg.Done()
			if err := AddNode(vm, joinCmd); err != nil {
				outErr := fmt.Errorf("AddNode %s, error: %w", vm.Name, err)
				beego.Error(outErr)
				errsMu.Lock()
				errs = append(errs, outErr)
				errsMu.Unlock()
			}
		}(vms[i])
	}
	wg.Wait() // wait for all nodes joined

	return errs
}

// delete a node from the Kubernetes cluster
func UninstallNode(name string) error {
	// // kubectl drain xxxxxx --ignore-daemonsets
	if err := DrainNode(name); err != nil {
		outErr := fmt.Errorf("DrainNode %s: error: %w", name, err)
		beego.Error(outErr)
		return outErr
	}

	// // put kubeconfig in /root/.kube
	sshPrivateKey := beego.AppConfig.String("k8sVmSshPrivateKey")
	sshPort := SshPort
	sshUser := SshRootUser

	node, err := GetNode(name, metav1.GetOptions{})
	if err != nil {
		outErr := fmt.Errorf("Kubernetes get node: error: %w", err)
		beego.Error(outErr)
		return outErr
	}
	nodeIp := GetNodeInternalIp(*node)

	// we try to do these behaviors, if not successful, we continue to do the following behaviors, to enable deleting node with problems
	func() {
		sshClient, err := SshClientWithPem(sshPrivateKey, sshUser, nodeIp, sshPort)
		if err != nil {
			outErr := fmt.Errorf("Create ssh client with SSH key fail: error: %w", err)
			beego.Error(outErr)
			return
		}
		defer sshClient.Close()

		// create the folder at the destination VM
		if _, err := SshOneCommand(sshClient, fmt.Sprintf("mkdir -p /root/.kube")); err != nil {
			outErr := fmt.Errorf("ssh error: %w", err)
			beego.Error(outErr)
			return
		}

		// put the kubeConfig into the folder of the destination VM
		var kubeConfigPath string
		if KubeConfigPath == "" {
			kubeConfigPath = defaultKubeConfigPath
		} else {
			kubeConfigPath = KubeConfigPath
		}
		if err := SftpCopyFile(kubeConfigPath, defaultKubeConfigPath, sshClient); err != nil {
			outErr := fmt.Errorf("SFTP error: %w", err)
			beego.Error(outErr)
			return
		}

		// // kubeadm reset the destination VM
		if _, err := SshOneCommand(sshClient, fmt.Sprintf("kubeadm reset -f")); err != nil {
			outErr := fmt.Errorf("ssh error: %w", err)
			beego.Error(outErr)
			return
		}

		// // Delete the folder /root/.kube
		if _, err := SshOneCommand(sshClient, fmt.Sprintf("rm -rf /root/.kube")); err != nil {
			outErr := fmt.Errorf("ssh error: %w", err)
			beego.Error(outErr)
			return
		}
	}()

	// // delete the node from the Kubernetes Cluster
	beego.Info(fmt.Sprintf("Use client-go API to delete node %s in Kubernetes cluster", name))
	if err := DeleteNode(name, metav1.DeleteOptions{}); err != nil {
		outErr := fmt.Errorf("use client-go API to delete node error: %w", err)
		beego.Error(outErr)
		return outErr
	}

	return nil
}

// delete nodes from the Kubernetes cluster concurrently
func UninstallBatchNodes(nodeNames []string) []error {
	var errs []error
	var errsMu sync.Mutex // the slice in golang is not safe for concurrent read/write

	// uninstall nodes in parallel
	var wg sync.WaitGroup

	for _, nodeName := range nodeNames {
		wg.Add(1)
		go func(nn string) {
			defer wg.Done()
			err := UninstallNode(nn)
			if err != nil {
				outErr := fmt.Errorf("uninstall node [%s], error %w.", nn, err)
				beego.Error(outErr)
				errsMu.Lock()
				errs = append(errs, outErr)
				errsMu.Unlock()
			}
		}(nodeName)
	}
	wg.Wait()

	return errs
}
