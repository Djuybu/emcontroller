# Kịch bản Nhiệt độ và Hao hụt trong Thực nghiệm Response Time

## Tổng quan

Đã thêm tính năng theo dõi nhiệt độ và tính toán hao hụt vào thực nghiệm response time (thực nghiệm 1). Khi nhiệt độ thay đổi, hệ thống sẽ tự động tính toán:
- **Performance Loss**: Mất mát hiệu suất do nhiệt độ
- **Power Overhead**: Tăng tiêu thụ điện năng do làm mát/sưởi ấm

## Các thay đổi chính

### 1. Cấu trúc dữ liệu (`data_types.py`)

Thêm 3 trường mới vào `ResultData`:
- `temperature`: Nhiệt độ tại cloud nơi app được triển khai (°C)
- `performance_loss`: Hệ số mất mát hiệu suất (0.0-1.0)
- `power_overhead`: Tăng tiêu thụ điện năng (%)

### 2. Lấy nhiệt độ từ Weather API (`cloud.go`)

- Hàm `GenerateOneCloud()` được cập nhật để lấy nhiệt độ từ Weather API
- Sử dụng mapping location mặc định cho các cloud (có thể mở rộng)
- Nếu không lấy được nhiệt độ, sử dụng giá trị mặc định 20°C

### 3. Tính toán hao hụt (`http_api.py`)

Hàm `calculate_temperature_losses()` tính toán:

#### Performance Loss (Mất mát hiệu suất):
- **20-25°C**: Không mất mát (0%)
- **Dưới 20°C**: Mất mát tuyến tính, tối đa 5% ở 0°C
- **Trên 25°C**: Mất mát theo hàm mũ:
  - 35°C: ~10% mất mát
  - 45°C: ~25% mất mát
  - 55°C: ~50% mất mát
  - Tối đa: 70% mất mát

#### Power Overhead (Tăng tiêu thụ điện):
- **20-25°C**: Không tăng (0%)
- **Dưới 20°C**: Cần sưởi ấm, tăng ~2% mỗi 5°C dưới 20°C
- **Trên 25°C**: Cần làm mát, tăng theo hàm mũ:
  - 30°C: ~5% tăng
  - 40°C: ~15% tăng
  - 50°C: ~40% tăng
  - Tối đa: 50% tăng

### 4. Lưu trữ dữ liệu (`csv_operation.py`)

- CSV files giờ bao gồm 3 cột mới: `temperature`, `performance_loss`, `power_overhead`
- Hỗ trợ đọc cả format cũ (không có nhiệt độ) và format mới

### 5. Biểu đồ (`charts_drawer.py`)

Thêm các metric mới vào biểu đồ:
- Temperature distribution
- Performance loss analysis
- Power overhead analysis

## Mapping Location Cloud

Hiện tại sử dụng mapping mặc định:
- `myvm`: Hà Nội (21.0285, 105.8542)
- `CLAAUDIAweifan`: Copenhagen (55.6762, 12.5683)
- Các cloud khác: Mặc định Hà Nội

Có thể mở rộng bằng cách:
1. Thêm vào `getCloudLocation()` trong `cloud.go`
2. Hoặc thêm vào file cấu hình `iaas.json`

## Cách sử dụng

### Chạy thực nghiệm

1. Chạy `go run init.go` để tạo dữ liệu thực nghiệm
2. Chạy `bash auto_deploy_call.sh` để triển khai và gọi apps
3. Chạy `python charts_drawer.py` để vẽ biểu đồ

### Xem kết quả

Dữ liệu CSV sẽ bao gồm:
- Response time metrics (như cũ)
- Temperature tại cloud
- Performance loss do nhiệt độ
- Power overhead do nhiệt độ

Biểu đồ sẽ hiển thị:
- Phân phối nhiệt độ
- Ảnh hưởng của nhiệt độ đến performance
- Ảnh hưởng của nhiệt độ đến power consumption

## Kịch bản nghiên cứu

### Kịch bản 1: Nhiệt độ cao
- Khi nhiệt độ > 35°C: Performance giảm đáng kể, power overhead tăng
- Có thể cần điều chỉnh scheduling để tránh cloud nóng

### Kịch bản 2: Nhiệt độ thấp
- Khi nhiệt độ < 20°C: Cần sưởi ấm, tăng power overhead
- Performance có thể giảm nhẹ

### Kịch bản 3: Tối ưu hóa
- Scheduling algorithms có thể xem xét nhiệt độ khi quyết định
- Ưu tiên cloud có nhiệt độ tối ưu (20-25°C)
- Tránh cloud quá nóng hoặc quá lạnh

## Mở rộng trong tương lai

1. **Dynamic location mapping**: Lấy location từ cloud metadata
2. **Real-time temperature**: Cập nhật nhiệt độ theo thời gian thực
3. **Temperature-aware scheduling**: Thuật toán scheduling xem xét nhiệt độ
4. **Cost analysis**: Tính toán chi phí dựa trên power overhead
5. **Predictive models**: Dự đoán nhiệt độ và điều chỉnh trước

