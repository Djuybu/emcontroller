# Multi-Cloud Manager #

## Thông tin cơ bản

Dự án này được phát triển và kiểm thử với `Go version go1.20.2 linux/amd64`.

Dự án được phát triển sử dụng framework Beego, với các API được định nghĩa trong `routers` và `controllers`, chức năng cốt lõi được mô tả trong thư mục `models`, và mã front-end nằm trong thư mục `views` và `static`. Người dùng cần cấu hình các tham số trong thư mục `conf`.

### Cách build Multi-cloud Manager và xóa các file đã build? ###

* `make` hoặc `make emcontroller` sẽ tạo file binary `emcontroller`.
* `make clean` sẽ xóa file binary `emcontroller`.

### Cách chạy và dừng Multi-Cloud Manager? ###

* Sau khi build file binary `emcontroller`, tại đường dẫn gốc của dự án, thực thi `./emcontroller`.
* `Ctrl + C` để dừng.

### Cách đặt Multi-cloud Manager làm service của systemd và xóa service? ###

* Sau khi build file binary `emcontroller`, tại đường dẫn gốc của dự án, thực thi `bash install_service.sh`.
* Thực thi `bash uninstall_service.sh` để xóa service.


## Lập lịch tự động
Multi-cloud Manager cho phép lập lịch các ứng dụng, như được mô tả chi tiết trong bài báo "_Multi-cloud Containerized Service Scheduling Optimizing Computation and Communication_". Chức năng này yêu cầu thông tin về Thời gian Round-Trip của Mạng (RTT) giữa các cặp cloud. Để hỗ trợ điều này, người dùng cần upload "container image kiểm tra hiệu năng mạng" vào kho lưu trữ container image. Multi-cloud Manager sử dụng tác vụ định kỳ để thu thập dữ liệu RTT.

### Cách tạo container image kiểm tra hiệu năng mạng? ###
1. Đặt thư mục `net-perf-container-image` vào một VM đã cài đặt Docker.
2. Trên VM đó, `cd` vào thư mục `net-perf-container-image`, và thực thi `docker build -t mcnettest:latest .`.

Mã nguồn cho lập lịch tự động có thể tìm thấy trong thư mục `auto-schedule`. Cụ thể, các thuật toán lập lịch được sử dụng trong phần "Evaluation" của bài báo được triển khai trong các file sau trong thư mục `auto-schedule/algorithms`: `mcssga.go`, `for_cmp_amaga.go`, `for_cmp_ampga.go`, `for_cmp_best_effort_rand.go`, và `for_cmp_diktyo_ga.go`.

Đối với "Dummy Service" được thảo luận trong bài báo, bạn có thể tìm mã nguồn của nó trong thư mục `auto-schedule/experiments/server`. Hơn nữa, các tham số dịch vụ (ví dụ: yêu cầu và các tham số khác) được sử dụng trong các thí nghiệm của bài báo được tạo bằng mã nguồn có sẵn trong thư mục `auto-schedule/experiments/applications-generator`. Để truy cập mã nguồn cụ thể cho hai thí nghiệm, vui lòng điều hướng đến các thư mục `auto-schedule/experiments/usable-accept-rate` và `auto-schedule/experiments/response-time`. Bạn sẽ tìm thấy thông tin chi tiết trong file `README.md` trong mỗi thư mục tương ứng.

## Dữ liệu của các thí nghiệm trong bài báo "_Multi-cloud Containerized Service Scheduling Optimizing Computation and Communication_"
- Dữ liệu của các thí nghiệm về Thời gian Lập lịch, Tỷ lệ Giải pháp Khả dụng, và Tỷ lệ Chấp nhận Dịch vụ là các file `.csv` trong thư mục `auto-schedule/experiments/usable-accept-rate`.
- Dữ liệu và nhóm dịch vụ của các thí nghiệm về Thời gian Phản hồi nằm trong thư mục `auto-schedule/experiments/response-time/executor-python/data`.
  - Các nhóm dịch vụ là các file `request_applications.json`.
  - Dữ liệu là các file `.csv`.