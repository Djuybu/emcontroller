package models

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego"
)

// ===================== Local driver (libvirt/virsh) =====================

type Local struct {
	Name               string
	IP                 string
	User               string
	KeyPath            string
	Network            string
	ImagePath          string
	PoolDir            string
	ImageInstallMethod string
	LibvirtURI         string

	Password   string
	SshPubKey  string
	StaticIP   string
	FixedMAC   string
	DHCPStatic bool

	// NEW: tọa độ để gọi API thời tiết
	Latitude  string
	Longitude string
}

// ===================== Exec helpers =====================

func runCmd(ctx context.Context, bin string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s %v failed: %v; stderr=%s", bin, args, err, stderr.String())
	}
	return stdout.String(), nil
}

func virshCmd(ctx context.Context, uri string, args ...string) (string, error) {
	full := []string{}
	if uri != "" {
		full = append(full, "--connect", uri)
	}
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, "virsh", full...)
	// Ép locale về tiếng Anh để parser ổn định
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("virsh %v error: %w (out=%s)", full, err, string(out))
	}
	return string(out), nil
}

func (l *Local) LibvirtURIOrDefault() string {
	if strings.TrimSpace(l.LibvirtURI) != "" {
		return l.LibvirtURI
	}
	return "qemu:///system"
}

// ===================== Parsers =====================

var (
	// dominfo
	reCPU     = regexp.MustCompile(`(?i)^\s*CPU\(s\):\s*([0-9]+)\s*$`)
	reMaxMem  = regexp.MustCompile(`(?i)^\s*Max memory:\s*([0-9]+)\s*([KMGT]i?B)\s*$`)
	reUsedMem = regexp.MustCompile(`(?i)^\s*Used memory:\s*([0-9]+)\s*([KMGT]i?B)\s*$`)

	// domblkinfo: hỗ trợ 3 dạng: số trần | số + bytes | số + đơn vị
	reBlkKVOnly  = regexp.MustCompile(`(?i)^\s*(Capacity|Allocation|Physical):\s*([0-9]+)\s*$`)
	reBlkKVBytes = regexp.MustCompile(`(?i)^\s*(Capacity|Allocation|Physical):\s*([0-9]+)\s+bytes\s*$`)
	reBlkKVUnit  = regexp.MustCompile(`(?i)^\s*(Capacity|Allocation|Physical):\s*([0-9]+)\s*([KMGT]i?B)\s*$`)
)

func toKiB(n uint64, unit string) uint64 {
	u := strings.ToUpper(unit)
	switch u {
	case "KIB", "KB":
		return n
	case "MIB", "MB":
		return n * 1024
	case "GIB", "GB":
		return n * 1024 * 1024
	case "TIB", "TB":
		return n * 1024 * 1024 * 1024
	default:
		return n
	}
}

func toBytes(n uint64, unit string) uint64 {
	u := strings.ToUpper(unit)
	switch u {
	case "KIB", "KB":
		return n * 1024
	case "MIB", "MB":
		return n * 1024 * 1024
	case "GIB", "GB":
		return n * 1024 * 1024 * 1024
	case "TIB", "TB":
		return n * 1024 * 1024 * 1024 * 1024
	default:
		return n
	}
}

// parseDomInfo → (vCPU, MaxMemKiB, UsedMemKiB)
func parseDomInfo(out string) (int, uint64, uint64) {
	var vcpu int
	var maxKiB, usedKiB uint64
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if m := reCPU.FindStringSubmatch(ln); m != nil {
			vcpu, _ = strconv.Atoi(m[1])
			continue
		}
		if m := reMaxMem.FindStringSubmatch(ln); m != nil {
			val, _ := strconv.ParseUint(m[1], 10, 64)
			maxKiB = toKiB(val, m[2])
			continue
		}
		if m := reUsedMem.FindStringSubmatch(ln); m != nil {
			val, _ := strconv.ParseUint(m[1], 10, 64)
			usedKiB = toKiB(val, m[2])
			continue
		}
	}
	return vcpu, maxKiB, usedKiB
}

// parseDomBlkInfo: trả về (capacity, allocation, physical) theo bytes
func parseDomBlkInfo(out string) (capB, allocB, physB uint64) {
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)

		if m := reBlkKVOnly.FindStringSubmatch(ln); m != nil {
			val, _ := strconv.ParseUint(m[2], 10, 64)
			switch strings.ToLower(m[1]) {
			case "capacity":
				capB = val
			case "allocation":
				allocB = val
			case "physical":
				physB = val
			}
			continue
		}

		if m := reBlkKVBytes.FindStringSubmatch(ln); m != nil {
			val, _ := strconv.ParseUint(m[2], 10, 64)
			switch strings.ToLower(m[1]) {
			case "capacity":
				capB = val
			case "allocation":
				allocB = val
			case "physical":
				physB = val
			}
			continue
		}

		if m := reBlkKVUnit.FindStringSubmatch(ln); m != nil {
			val, _ := strconv.ParseUint(m[2], 10, 64)
			b := toBytes(val, m[3])
			switch strings.ToLower(m[1]) {
			case "capacity":
				capB = b
			case "allocation":
				allocB = b
			case "physical":
				physB = b
			}
			continue
		}
	}
	return
}

func parseQemuImgVirtualSize(jsonStr string) uint64 {
	type qi struct {
		VirtualSize uint64 `json:"virtual-size"`
	}
	var v qi
	_ = json.Unmarshal([]byte(jsonStr), &v)
	return v.VirtualSize
}

func parseDomIfAddr(out string) []string {
	res := []string{}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "Name") || strings.HasPrefix(ln, "lo") {
			continue
		}
		for _, tok := range strings.Fields(ln) {
			// IPv4 A.B.C.D/xx hoặc IPv6 ::/xx
			if strings.Contains(tok, "/") && (strings.Count(tok, ".") == 3 || strings.Contains(tok, ":")) {
				res = append(res, strings.SplitN(tok, "/", 2)[0])
			}
		}
	}
	seen := map[string]bool{}
	uniq := []string{}
	for _, ip := range res {
		if !seen[ip] {
			seen[ip] = true
			uniq = append(uniq, ip)
		}
	}
	return uniq
}

// ===================== Host resource helpers =====================

func (l *Local) readNodeInfo() (cpuTotal int, ramTotalMB int, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "nodeinfo")
	if err != nil {
		return 0, 0, err
	}
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "CPU(s):") {
			fmt.Sscanf(ln, "CPU(s): %d", &cpuTotal)
		} else if strings.HasPrefix(ln, "Memory size:") {
			var memKiB int
			fmt.Sscanf(ln, "Memory size: %d KiB", &memKiB)
			ramTotalMB = memKiB / 1024
		}
	}
	return
}

func (l *Local) readDiskUsage() (totalGB, usedGB float64, err error) {
	if l.PoolDir == "" {
		l.PoolDir = "/var/lib/libvirt/images"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	out, err := runCmd(ctx, "df", "-B1", l.PoolDir)
	if err != nil {
		return 0, 0, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return 0, 0, fmt.Errorf("unexpected df output")
	}
	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("unexpected df fields")
	}
	parseF := func(s string) float64 {
		v, _ := strconv.ParseFloat(s, 64)
		return v
	}
	total := parseF(fields[1])
	used := parseF(fields[2])
	gb := 1024.0 * 1024.0 * 1024.0
	return total / gb, used / gb, nil
}

func (l *Local) readDiskSizeGBByPath(diskPath string) (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := runCmd(ctx, "qemu-img", "info", "--output=json", diskPath)
	if err != nil {
		// đang chạy/lock → thử --force-share
		out2, err2 := runCmd(ctx, "qemu-img", "info", "--output=json", "--force-share", diskPath)
		if err2 != nil {
			return 0, err
		}
		var ii2 struct {
			VirtualSize float64 `json:"virtual-size"`
		}
		if err := json.Unmarshal([]byte(out2), &ii2); err != nil {
			return 0, err
		}
		return ii2.VirtualSize / (1024 * 1024 * 1024), nil
	}
	var ii struct {
		VirtualSize float64 `json:"virtual-size"`
	}
	if err := json.Unmarshal([]byte(out), &ii); err != nil {
		return 0, err
	}
	return ii.VirtualSize / (1024 * 1024 * 1024), nil
}

// ===================== Iaas interface: methods =====================

func (l *Local) ShowName() string   { return l.Name }
func (l *Local) ShowType() string   { return LocalIaas } // "local"
func (l *Local) ShowWebUrl() string { return l.IP }

// GetVM: trả chi tiết 1 VM
func (l *Local) GetVM(vmID string) (*IaasVm, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uri := l.LibvirtURIOrDefault()

	// State
	stOut, err := virshCmd(ctx, uri, "domstate", vmID)
	if err != nil {
		return nil, fmt.Errorf("domstate %s error: %w", vmID, err)
	}
	rawState := strings.ToLower(strings.TrimSpace(stOut))
	status := "Pending"
	switch {
	case strings.Contains(rawState, "running"):
		status = "Running"
	case strings.Contains(rawState, "shut"):
		status = "Shut Off"
	case strings.Contains(rawState, "paused"):
		status = "Paused"
	}

	// vCPU / RAM
	infoOut, err := virshCmd(ctx, uri, "dominfo", vmID)
	if err != nil {
		return nil, fmt.Errorf("dominfo %s error: %w", vmID, err)
	}
	vcpu, maxKiB, usedKiB := parseDomInfo(infoOut)
	ramKiB := maxKiB
	if usedKiB > 0 {
		ramKiB = usedKiB
	}

	// Disk (capacity)
	blkOut, err := virshCmd(ctx, uri, "domblklist", "--details", vmID)
	if err != nil {
		return nil, fmt.Errorf("domblklist %s error: %w", vmID, err)
	}
	target, srcPath := resolveTargetFromDomBlkList(blkOut)

	var virtBytes uint64
	if target != "" && srcPath != "" {
		if status == "Running" {
			if biOut, err := virshCmd(ctx, uri, "domblkinfo", vmID, target); err == nil {
				if capB, _, _ := parseDomBlkInfo(biOut); capB > 0 {
					virtBytes = capB
				}
			}
			if virtBytes == 0 {
				if qiOut, err := runCmd(ctx, "qemu-img", "info", "--output=json", "--force-share", srcPath); err == nil {
					virtBytes = parseQemuImgVirtualSize(qiOut)
				}
			}
		} else {
			if qiOut, err := runCmd(ctx, "qemu-img", "info", "--output=json", srcPath); err == nil {
				virtBytes = parseQemuImgVirtualSize(qiOut)
			} else if qiOut2, err2 := runCmd(ctx, "qemu-img", "info", "--output=json", "--force-share", srcPath); err2 == nil {
				virtBytes = parseQemuImgVirtualSize(qiOut2)
			}
		}
	}

	// IPs
	ips := []string{}
	if ifOut, err := virshCmd(ctx, uri, "domifaddr", vmID, "--full", "--source", "agent"); err == nil {
		ips = parseDomIfAddr(ifOut)
	}
	if len(ips) == 0 {
		if ifOut2, err2 := virshCmd(ctx, uri, "domifaddr", vmID, "--full"); err2 == nil {
			ips = parseDomIfAddr(ifOut2)
		}
	}
	if len(ips) == 0 {
		if ip, _ := l.ipFromLease(ctx, vmID); ip != "" {
			ips = []string{ip}
		}
	}

	return &IaasVm{
		ID:        vmID,
		Name:      vmID,
		IPs:       ips,
		VCpu:      float64(vcpu),
		Ram:       float64(ramKiB) / 1024.0,                   // KiB → MB
		Storage:   float64((virtBytes + (1 << 30) - 1) >> 30), // bytes → GB (ceil)
		Status:    status,
		Cloud:     l.Name,
		CloudType: LocalIaas,
		McmCreate: true, // tuỳ logic của bạn
	}, nil
}

// ListAllVMs: gộp qemu:///system và qemu:///session, dùng getVMFromURI
func (l *Local) ListAllVMs() ([]IaasVm, error) {
	vms := []IaasVm{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	namesSys, errSys := listNamesByURI(ctx, "qemu:///system")
	if errSys != nil {
		beego.Warn(fmt.Sprintf("virsh system error: %v", errSys))
	}

	namesSes := []string{}
	if len(namesSys) == 0 {
		if ns, errSes := listNamesByURI(ctx, "qemu:///session"); errSes != nil {
			if errSys != nil {
				return nil, fmt.Errorf("virsh list failed: system=%v, session=%v", errSys, errSes)
			}
			beego.Warn(fmt.Sprintf("virsh session error: %v", errSes))
		} else {
			namesSes = ns
		}
	}

	seen := map[string]bool{}
	for _, n := range namesSys {
		seen[n] = true
		if vm, err := l.getVMFromURI(ctx, n, "qemu:///system"); err != nil {
			beego.Warn(fmt.Sprintf("Skip VM %s (system): %v", n, err))
		} else {
			vms = append(vms, *vm)
		}
	}
	for _, n := range namesSes {
		if seen[n] {
			continue
		}
		if vm, err := l.getVMFromURI(ctx, n, "qemu:///session"); err != nil {
			beego.Warn(fmt.Sprintf("Skip VM %s (session): %v", n, err))
		} else {
			vms = append(vms, *vm)
		}
	}

	if len(vms) == 0 {
		beego.Info("No VMs found in both qemu:///system and qemu:///session")
	}
	return vms, nil
}

// CreateVM: tạo VM cơ bản qua virt-install (import nếu có ImagePath)
func (l *Local) CreateVM(name string, vcpu, ramMB, storageGB int) (*IaasVm, error) {
	if vcpu <= 0 {
		vcpu = 2
	}
	if ramMB <= 0 {
		ramMB = 1024
	}
	if storageGB <= 0 {
		storageGB = 10
	}
	if l.Network == "" {
		l.Network = "default"
	}
	if l.PoolDir == "" {
		l.PoolDir = "/var/lib/libvirt/images"
	}

	diskPath, err := l.makeDiskFor(name, storageGB, l.ImagePath)
	if err != nil {
		return nil, err
	}

	args := []string{
		"--name", name,
		"--memory", fmt.Sprint(ramMB),
		"--vcpus", fmt.Sprint(vcpu),
		"--disk", fmt.Sprintf("path=%s,format=qcow2,bus=virtio", diskPath),
		"--network", fmt.Sprintf("network=%s,model=virtio", l.Network),
		"--graphics", "none",
		"--noautoconsole",
		"--os-variant", "generic",
	}
	if l.ImagePath != "" {
		args = append(args, "--import", "--boot", "hd")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := runCmd(ctx, "virt-install", args...); err != nil {
		return nil, err
	}

	// Lấy IP best-effort
	ip := ""
	if v, err := l.ipFromLease(context.Background(), name); err == nil {
		ip = v
	}

	return &IaasVm{
		ID:        name,
		Name:      name,
		IPs:       condIP(ip),
		VCpu:      float64(vcpu),
		Ram:       float64(ramMB),
		Storage:   float64(storageGB),
		Status:    "Running",
		Cloud:     l.Name,
		CloudType: LocalIaas,
		McmCreate: true,
	}, nil
}

// DeleteVM: destroy + undefine --remove-all-storage
func (l *Local) DeleteVM(vmID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _ = virshCmd(ctx, l.LibvirtURIOrDefault(), "destroy", vmID) // ignore nếu không chạy
	if _, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "undefine", vmID, "--remove-all-storage"); err != nil {
		return fmt.Errorf("delete VM %s failed: %v", vmID, err)
	}
	return nil
}

// CheckResources: tổng hợp limit/in-use (CPU, RAM, Storage, VM)
func (l *Local) CheckResources() (ResourceStatus, error) {
	rs := ResourceStatus{Limit: ResSet{}, InUse: ResSet{}}

	// 1) Limit host
	if cpuTotal, ramTotalMB, err := l.readNodeInfo(); err == nil {
		if cpuTotal > 0 {
			rs.Limit.VCpu = float64(cpuTotal)
		}
		if ramTotalMB > 0 {
			rs.Limit.Ram = float64(ramTotalMB)
		}
	}
	if stTotal, stUsed, err := l.readDiskUsage(); err == nil {
		rs.Limit.Storage = stTotal
		rs.InUse.Storage = stUsed
	}

	// 2) InUse từ VM
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	out, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "list", "--all")
	if err != nil {
		return rs, nil // giữ 0 nếu lỗi
	}
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) >= 3 && f[0] != "Id" && f[0] != "-" && f[1] != "Name" {
			name := f[1]

			if inf, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "dominfo", name); err == nil {
				vcpu, maxKiB, _ := parseDomInfo(inf)
				rs.InUse.VCpu += float64(vcpu)
				rs.InUse.Ram += float64(maxKiB) / 1024.0 // KiB → MB
			}
			if blk, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "domblklist", "--details", name); err == nil {
				_, src := resolveTargetFromDomBlkList(blk)
				if src != "" {
					if sz, e := l.readDiskSizeGBByPath(src); e == nil {
						rs.InUse.Storage += sz
					}
				}
			}
			rs.InUse.Vm++
		}
	}
	return rs, nil
}

// IsCreatedByMcm: tùy logic; ở đây coi VM local là do MCM tạo
func (l *Local) IsCreatedByMcm(vmID string) (bool, error) {
	return true, nil
}

// ===================== VM enumeration helper =====================

func listNamesByURI(ctx context.Context, uri string) ([]string, error) {
	out, err := virshCmd(ctx, uri, "list", "--all")
	if err != nil {
		return nil, err
	}
	var names []string
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, ln := range lines {
		f := strings.Fields(ln)
		if len(f) >= 3 && f[0] != "Id" && f[0] != "-" && f[1] != "Name" {
			names = append(names, f[1])
		}
	}
	return names, nil
}

func resolveTargetFromDomBlkList(out string) (target, srcPath string) {
	bestTarget, bestSrc := "", ""
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, ln := range lines {
		f := strings.Fields(ln)
		// TYPE DEVICE TARGET SOURCE
		if len(f) < 4 {
			continue
		}
		typ, dev, tgt, src := f[0], f[1], f[2], f[3]
		if typ != "file" || dev != "disk" {
			continue
		}
		if src == "" || strings.HasSuffix(strings.ToLower(src), ".iso") {
			continue
		}
		bestTarget, bestSrc = tgt, src
		break
	}
	return bestTarget, bestSrc
}

func (l *Local) getVMFromURI(ctx context.Context, name, uri string) (*IaasVm, error) {
	// 1) State
	stOut, err := virshCmd(ctx, uri, "domstate", name)
	if err != nil {
		return nil, fmt.Errorf("domstate error: %w", err)
	}
	rawState := strings.ToLower(strings.TrimSpace(stOut))
	status := "Pending"
	switch {
	case strings.Contains(rawState, "running"):
		status = "Running"
	case strings.Contains(rawState, "shut"):
		status = "Shut Off"
	case strings.Contains(rawState, "paused"):
		status = "Paused"
	}

	// 2) vCPU / RAM
	infoOut, err := virshCmd(ctx, uri, "dominfo", name)
	if err != nil {
		return nil, fmt.Errorf("dominfo error: %w", err)
	}
	vcpu, maxKiB, usedKiB := parseDomInfo(infoOut)
	ramKiB := maxKiB
	if usedKiB > 0 {
		ramKiB = usedKiB
	}

	// 3) Disk (capacity)
	blkOut, err := virshCmd(ctx, uri, "domblklist", "--details", name)
	if err != nil {
		return nil, fmt.Errorf("domblklist error: %w", err)
	}
	target, srcPath := resolveTargetFromDomBlkList(blkOut)
	var virtBytes uint64
	if target != "" && srcPath != "" {
		if status == "Running" {
			if biOut, err := virshCmd(ctx, uri, "domblkinfo", name, target); err == nil {
				if capB, _, _ := parseDomBlkInfo(biOut); capB > 0 {
					virtBytes = capB
				}
			}
			if virtBytes == 0 {
				if qiOut, err := runCmd(ctx, "qemu-img", "info", "--output=json", "--force-share", srcPath); err == nil {
					virtBytes = parseQemuImgVirtualSize(qiOut)
				}
			}
		} else {
			if qiOut, err := runCmd(ctx, "qemu-img", "info", "--output=json", srcPath); err == nil {
				virtBytes = parseQemuImgVirtualSize(qiOut)
			} else if qiOut2, err2 := runCmd(ctx, "qemu-img", "info", "--output=json", "--force-share", srcPath); err2 == nil {
				virtBytes = parseQemuImgVirtualSize(qiOut2)
			}
		}
	}

	// 4) IPs
	ips := []string{}
	if ifOut, err := virshCmd(ctx, uri, "domifaddr", name, "--full", "--source", "agent"); err == nil {
		ips = parseDomIfAddr(ifOut)
	}
	if len(ips) == 0 {
		if ifOut2, err2 := virshCmd(ctx, uri, "domifaddr", name, "--full"); err2 == nil {
			ips = parseDomIfAddr(ifOut2)
		}
	}
	if len(ips) == 0 {
		if ip, _ := l.ipFromLease(ctx, name); ip != "" {
			ips = []string{ip}
		}
	}

	vm := &IaasVm{
		ID:        name,
		Name:      name,
		Cloud:     l.Name,
		CloudType: LocalIaas,
		McmCreate: true,

		IPs:     ips,
		VCpu:    float64(vcpu),
		Ram:     float64(ramKiB) / 1024.0,                   // KiB → MB
		Storage: float64((virtBytes + (1 << 30) - 1) >> 30), // bytes → GB (ceil)
		Status:  status,
	}
	return vm, nil
}

// ===================== Disk & Network helpers =====================

func (l *Local) makeDiskFor(name string, sizeGB int, backingImg string) (string, error) {
	if sizeGB <= 0 {
		sizeGB = 10
	}
	if l.PoolDir == "" {
		l.PoolDir = "/var/lib/libvirt/images"
	}
	if err := os.MkdirAll(l.PoolDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir pool dir failed: %w", err)
	}
	dst := filepath.Join(l.PoolDir, fmt.Sprintf("%s.qcow2", name))
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if strings.TrimSpace(backingImg) != "" {
		if _, err := runCmd(ctx, "qemu-img", "create", "-f", "qcow2", "-F", "qcow2", "-b", backingImg, dst, fmt.Sprintf("%dG", sizeGB)); err != nil {
			return "", err
		}
	} else {
		if _, err := runCmd(ctx, "qemu-img", "create", "-f", "qcow2", dst, fmt.Sprintf("%dG", sizeGB)); err != nil {
			return "", err
		}
	}
	return dst, nil
}

func (l *Local) getMAC(ctx context.Context, name string) (string, string, error) {
	out, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "domiflist", name)
	if err != nil {
		return "", "", err
	}
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		// Name  Type     Source   Model   MAC
		if len(f) >= 5 && f[1] == "network" {
			return f[2], f[len(f)-1], nil // (networkName, mac)
		}
	}
	return "", "", fmt.Errorf("cannot find NIC for %s", name)
}

func (l *Local) ipFromLease(ctx context.Context, name string) (string, error) {
	netName, mac, err := l.getMAC(ctx, name)
	if err != nil {
		return "", err
	}
	out, err := virshCmd(ctx, l.LibvirtURIOrDefault(), "net-dhcp-leases", netName, "--mac", mac)
	if err != nil {
		return "", err
	}
	for _, ln := range strings.Split(out, "\n") {
		if strings.Contains(ln, mac) && strings.Contains(ln, "ipv4") {
			for _, p := range strings.Fields(ln) {
				if strings.Count(p, ".") == 3 && strings.Contains(p, "/") {
					return strings.SplitN(p, "/", 2)[0], nil
				}
			}
		}
	}
	return "", fmt.Errorf("no DHCP lease for %s", name)
}

// condIP: đảm bảo luôn trả slice hợp lệ
func condIP(ip string) []string {
	if ip == "" {
		return []string{}
	}
	return []string{ip}
}

// ===================== Factory =====================

func getStr(params map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := params[k]; ok && v != nil {
			return fmt.Sprint(v)
		}
	}
	return ""
}

// GetLatitude implements the locator interface for Local
func (l *Local) GetLatitude() string {
	return l.Latitude
}

// GetLongitude implements the locator interface for Local
func (l *Local) GetLongitude() string {
	return l.Longitude
}

func InitLocal(params map[string]interface{}) *Local {
	network := strings.TrimSpace(getStr(params, "network"))
	if network == "" {
		network = "default"
	}
	poolDir := strings.TrimSpace(getStr(params, "pool_dir", "poolDir"))
	if poolDir == "" {
		poolDir = "/var/lib/libvirt/images"
	}
	return &Local{
		Name:               getStr(params, "name"),
		IP:                 getStr(params, "ip"),
		User:               getStr(params, "user"),
		KeyPath:            getStr(params, "key_path", "keyPath"),
		Network:            network,
		ImagePath:          getStr(params, "image_path", "imagePath"),
		PoolDir:            poolDir,
		ImageInstallMethod: strings.ToLower(strings.TrimSpace(getStr(params, "image_install_method", "imageInstallMethod"))),
		Password:           getStr(params, "password"),
		SshPubKey:          getStr(params, "ssh_pub_key", "sshPubKey"),
		StaticIP:           getStr(params, "static_ip", "staticIP"),
		FixedMAC:           strings.ToLower(getStr(params, "fixed_mac", "fixedMAC")),
		DHCPStatic:         strings.EqualFold(getStr(params, "dhcp_static", "dhcpStatic"), "true"),
		LibvirtURI:         strings.TrimSpace(getStr(params, "libvirt_uri", "libvirtURI")),

		Latitude:  getStr(params, "latitude", "lat"),
		Longitude: getStr(params, "longitude", "lon", "longitude"),
	}
}
