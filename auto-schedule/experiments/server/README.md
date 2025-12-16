## Chương trình này dùng để làm gì?
Đây là một ứng dụng server cho các thí nghiệm về thời gian phản hồi.

Khi ứng dụng này chạy, nó sẽ chiếm các giá trị input của Storage và Memory. Khi nhận được yêu cầu API, nó sẽ gửi yêu cầu đến tất cả các ứng dụng phụ thuộc và đợi phản hồi, sau đó thực hiện workload của chính nó, và cuối cùng gửi phản hồi.

## Cách sử dụng chương trình?
### Cách build ứng dụng này?
1. Trong thư mục này, chạy `go build -o experiment-app`, sau đó file `experiment-app` sẽ được tạo ra. Đây là file binary thực thi của ứng dụng.
2. Chạy `docker build -t mcexp:latest .`, sau đó container image với "RepoTag" `mcexp:latest` sẽ được tạo.

### Cách chạy ứng dụng này?

Các tham số được mô tả trước hàm `main()` trong `main.go`.

Ví dụ gọi multi-cloud manager để chạy ứng dụng containerized này:
```shell
curl -i -X POST -H Content-Type:application/json -d '{"name":"exp-app2","replicas":1,"hostNetwork":false,"nodeName":"testmem","containers":[{"name":"exp-app2","image":"172.27.15.31:5000/mcexp:latest","workDir":"","resources":{"limits":{"memory":"5000Mi","cpu":"2","storage":"10Gi"},"requests":{"memory":"5000Mi","cpu":"0.5","storage":"10Gi"}},"commands":["./experiment-app"],"args":["5000000","2","5000","10","http://exp-app1-service:81/experiment"],"env":null,"mounts":null,"ports":[{"containerPort":3333,"name":"tcp","protocol":"tcp","servicePort":"81","nodePort":"30002"}]}],"priority":0,"autoScheduled":false}' http://172.27.15.31:20000/doNewApplication
```

### Cách gọi ứng dụng này?
Gửi yêu cầu HTTP `GET` đến port `:3333` và uri `/experiment`.
```shell
curl -i -X GET http://<IP>:3333/experiment
```
Ứng dụng sẽ đặt **thời gian tiêu thụ bởi clouds** trong response body, từ thời điểm ứng dụng nhận được yêu cầu đến khi gửi phản hồi. Đơn vị là `millisecond (ms)`.