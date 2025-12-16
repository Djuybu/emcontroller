## Chương trình này dùng để làm gì?
Thí nghiệm tự động về thời gian phản hồi.

## Cách thực thi thí nghiệm sử dụng chương trình này?

### Bước 1: Quyết định các tham số thí nghiệm

Cần quyết định các tham số sau:
- **repeat count**: Số lần lặp lại thí nghiệm
- **device count**: Số thiết bị gửi request (hiện tại nên để `1` để tự động)
- **app count**: Số lượng ứng dụng trong mỗi lần triển khai
- **request count per app**: Số lần request mỗi ứng dụng
- **multi-cloud manager endpoint**: Địa chỉ của Multi-cloud Manager
- **expt-app name prefix**: Tiền tố tên ứng dụng thí nghiệm
- **auto-scheduling VM name prefix**: Tiền tố tên VM cho auto-scheduling

Sau khi quyết định, cần cấu hình chúng trong các file liên quan:
- `init.go`
- `auto_deploy_call.sh`
- `caller.py`
- `deleter.py`
- `charts_drawer.py`
- `http_api.py`
- `auto-schedule/experiments/applications-generator/generator_test.go`

#### Giải thích các tham số:

**repeat count (Số lần lặp lại):**
Nên lặp lại thí nghiệm nhiều lần để giảm tác động của các yếu tố ngẫu nhiên.

**device count, app count, request count per app:**
- `app count`: Số lượng ứng dụng trong yêu cầu triển khai/lập lịch. Khi sử dụng `auto-schedule/experiments/applications-generator` để tạo yêu cầu triển khai, cần đặt giá trị này.
- `device count` và `request count per app`: Trong mỗi lần lặp, có thể sử dụng nhiều thiết bị để yêu cầu các ứng dụng nhằm mô phỏng môi trường production thực tế, và mỗi thiết bị có thể truy cập mỗi app nhiều lần để giảm tác động của các yếu tố ngẫu nhiên.

### Bước 2: Xóa dữ liệu cũ
- Di chuyển tất cả `executor-python/data/repeatX` vào thư mục `executor-python/data/bak`.
- Xóa tất cả `executor-python/data/repeatX`.

### Bước 3: Tạo yêu cầu triển khai và các thư mục cần thiết
Tại thư mục này, chạy:
```bash
go run init.go
```
Lệnh này sẽ tạo:
- File JSON chứa yêu cầu triển khai ứng dụng
- Các thư mục cần thiết cho việc lưu trữ kết quả

### Bước 4: Triển khai và gọi ứng dụng
Tại thư mục `executor-python`, chạy:
```bash
bash auto_deploy_call.sh
```

Script này sẽ:
- Triển khai các ứng dụng lên Multi-cloud Manager
- Tự động gọi các ứng dụng sau khi triển khai thành công
- Ghi lại thời gian phản hồi

**Chạy ở chế độ nền trên VM:**
```bash
nohup bash auto_deploy_call.sh 2>&1 &
```

### Bước 5: Vẽ biểu đồ
Trên hệ thống có GUI, tại thư mục `executor-python`:

**Vẽ biểu đồ CDF (Cumulative Distribution Function):**
```bash
python -u charts_drawer.py
```

**Vẽ biểu đồ điểm (Dot charts):**
```bash
python -u charts_drawer_no_cdf.py
```

## Cấu trúc thư mục dữ liệu
```
executor-python/data/
├── repeat1/
│   ├── {algorithm_name}/
│   │   └── *.csv (dữ liệu thời gian phản hồi)
│   └── request_applications.json
├── repeat2/
│   └── ...
└── bak/ (lưu trữ dữ liệu cũ)
```

## Lưu ý
- Đảm bảo Multi-cloud Manager đang chạy và có thể truy cập được
- Kiểm tra đủ tài nguyên cluster trước khi triển khai
- Thí nghiệm có thể mất nhiều thời gian tùy thuộc vào số lượng ứng dụng và số lần lặp