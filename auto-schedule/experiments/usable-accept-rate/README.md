## Chương trình này dùng để làm gì?
Thí nghiệm về:
1. Xác suất nhận được giải pháp không khả dụng.
2. Tỷ lệ chấp nhận ứng dụng.
3. Tỷ lệ chấp nhận ứng dụng của mỗi mức ưu tiên.
4. Thời gian lập lịch được sử dụng bởi các thuật toán khác nhau.

## Cách sử dụng chương trình?

### Bước 1: Cấu hình tham số
1. Đặt giá trị của các biến `appCounts` và `repeatCount` trong file `executor.go`. Đồng thời, đặt cùng giá trị số lượng ứng dụng cho hằng số `APP_COUNTS` trong file `common.py`.
   - `appCounts`: Số lượng ứng dụng cần kiểm thử (ví dụ: 40, 60, 80)
   - `repeatCount`: Số lần lặp lại thí nghiệm (ví dụ: 20 lần)

2. Trong file `auto-schedule/executors/create.go`, uncomment mã debug trong hàm `CreateAutoScheduleApps`, nhưng mã `draw evolution chart` vẫn nên được comment.

3. Trong file `auto-schedule/experiments/applications-generator/generator.go`, thay đổi `mcmEndpoint` thành `localhost:20000` để tránh phụ thuộc vào multi-cloud manager khác.

### Bước 2: Chuẩn bị môi trường
1. Đảm bảo database network state đang chạy (cần thiết cho việc lập lịch)
2. Chuẩn bị cấu hình Kubernetes tại `/root/.kube/config` (nếu chưa có)

### Bước 3: Chạy Multi-Cloud Manager
Tại thư mục gốc của dự án, chạy:
```bash
go run main.go
```
Multi-cloud manager sẽ chạy tại `localhost:20000` ở chế độ `debug`.

### Bước 4: Chạy thí nghiệm
Tại thư mục này (`auto-schedule/experiments/usable-accept-rate`), chạy:
```bash
go run executor.go
```

Thí nghiệm sẽ:
- Tự động tạo các nhóm ứng dụng với số lượng khác nhau
- Kiểm thử từng thuật toán lập lịch (CompRand, BERand, Amaga, Ampga, Diktyoga, Mcssga)
- Lặp lại mỗi thí nghiệm `repeatCount` lần
- Ghi kết quả vào file `usable_acceptance_rate_<appCount>.csv`

### Bước 5 (Tùy chọn): Chạy trên VM production
Để chạy trong môi trường gần với production thực tế:

1. **Biên dịch mã nguồn:**
   ```bash
   # Tại thư mục gốc
   make
   # Tại thư mục usable-accept-rate
   go build -o usable-accept-rate executor.go
   ```

2. **Sao chép lên VM:**
   ```bash
   # Sao chép toàn bộ dự án
   scp -r <project_root> user@vm:/path/to/destination
   # Sao chép cấu hình Kubernetes
   scp -r /root/.kube user@vm:/root/
   ```

3. **Chạy trên VM:**
   ```bash
   # Chạy Multi-cloud manager ở nền
   nohup ./emcontroller > emcontroller.log 2>&1 &
   # Chạy thí nghiệm ở nền
   cd auto-schedule/experiments/usable-accept-rate
   nohup ./usable-accept-rate > experiment.log 2>&1 &
   ```

4. **Lấy kết quả về:**
   ```bash
   scp user@vm:/path/to/usable_acceptance_rate_*.csv ./
   ```

### Bước 6: Vẽ biểu đồ
Trên máy tính có GUI, tại thư mục này:

**Biểu đồ tỷ lệ chấp nhận theo mức ưu tiên:**
```bash
python -u drawer_acc_rate.py
```
Tạo biểu đồ cột so sánh tỷ lệ chấp nhận ứng dụng của mỗi thuật toán cho từng mức ưu tiên, một biểu đồ cho mỗi số lượng ứng dụng.

**Biểu đồ tỷ lệ chấp nhận tổng thể:**
```bash
python -u drawer_total_acc_rate.py
```
Tạo biểu đồ cột so sánh tỷ lệ chấp nhận ứng dụng tổng thể của mỗi thuật toán với số lượng ứng dụng khác nhau.

**Biểu đồ thời gian lập lịch:**
```bash
python -u drawer_sched_time.py
```
Tạo biểu đồ cột so sánh thời gian lập lịch tối đa của mỗi thuật toán với số lượng ứng dụng khác nhau.

## Cấu trúc dữ liệu đầu ra
File CSV được tạo có cấu trúc:
- Algorithm Name: Tên thuật toán
- Maximum Scheduling Time (s): Thời gian lập lịch tối đa
- Usable Solution Rate: Tỷ lệ giải pháp khả dụng
- Application Acceptance Rate: Tỷ lệ chấp nhận ứng dụng
- Priority X Acceptance Rate: Tỷ lệ chấp nhận theo từng mức ưu tiên

## Lưu ý
- Đảm bảo Multi-cloud manager đang chạy trước khi bắt đầu thí nghiệm
- Thí nghiệm có thể mất nhiều thời gian tùy thuộc vào `appCounts` và `repeatCount`
- Cần đủ tài nguyên cluster để triển khai số lượng ứng dụng lớn