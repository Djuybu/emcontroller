## Chương trình này dùng để làm gì?
Trong các thí nghiệm của chúng ta, cần mô phỏng môi trường multi-cloud thực tế, và có độ trễ mạng giữa các cloud thực tế, vì vậy chúng ta tạo chương trình này để sử dụng TC (Traffic Control) để thêm độ trễ mạng giữa các cloud.

### Cách tạo độ trễ giữa các cloud?
1. Cấu hình các giá trị độ trễ trong hàm `TestGenCloudsDelay`.
2. Tại thư mục gốc của dự án, chạy:
```
go test <Thư mục Gốc Dự án>/auto-schedule/experiments/gen-net-delay/ -v -count=1 -run TestGenCloudsDelay
```
Ví dụ, nếu `<Thư mục Gốc Dự án>` là `/mnt/c/mine/code/gocode/src/emcontroller`, chúng ta nên thực thi:
```
go test /mnt/c/mine/code/gocode/src/emcontroller/auto-schedule/experiments/gen-net-delay/ -v -count=1 -run TestGenCloudsDelay
```

### Cách xóa tất cả độ trễ giữa các cloud?
Tại thư mục gốc của dự án, chạy:
```
go test <Thư mục Gốc Dự án>/auto-schedule/experiments/gen-net-delay/ -v -count=1 -run TestClearAllDelay
```
Ví dụ, nếu `<Thư mục Gốc Dự án>` là `/mnt/c/mine/code/gocode/src/emcontroller`, chúng ta nên thực thi:
```
go test /mnt/c/mine/code/gocode/src/emcontroller/auto-schedule/experiments/gen-net-delay/ -v -count=1 -run TestClearAllDelay
```
