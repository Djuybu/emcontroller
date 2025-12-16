# Tích hợp Nhiệt độ và Hao hụt vào Thực nghiệm Usable Acceptance Rate

## Tổng quan

Đã thêm tính năng theo dõi nhiệt độ và tính toán hao hụt vào thực nghiệm usable acceptance rate. Khi chạy thực nghiệm, hệ thống sẽ tự động:
1. Lấy nhiệt độ từ các cloud được sử dụng trong scheduling
2. Tính toán hao hụt hiệu suất (performance loss) và tăng tiêu thụ điện (power overhead)
3. Xuất kết quả vào CSV với các metrics mới

## Các thay đổi

### 1. Cấu trúc dữ liệu (`exptData`)

Thêm 4 trường mới:
- `avgTemperature`: Nhiệt độ trung bình tại các cloud được sử dụng (°C)
- `avgPerformanceLoss`: Mất mát hiệu suất trung bình (0.0-1.0)
- `avgPowerOverhead`: Tăng tiêu thụ điện trung bình (%)
- `temperatureCount`: Số lần đo nhiệt độ (để tính trung bình)

### 2. Tính toán nhiệt độ và hao hụt

#### Hàm `calculateTemperatureAndLosses()`
- Lấy thông tin tất cả clouds từ `GenerateClouds()`
- Tính nhiệt độ trung bình của tất cả clouds
- Tính hao hụt dựa trên nhiệt độ trung bình

#### Hàm `calculateLosses(temperature)`
Tính toán hao hụt dựa trên mô hình:

**Performance Loss (Mất mát hiệu suất):**
- **20-25°C**: Không mất mát (0%)
- **Dưới 20°C**: Mất mát tuyến tính, tối đa 5% ở 0°C
- **Trên 25°C**: Mất mát theo hàm mũ:
  - 35°C: ~10% mất mát
  - 45°C: ~25% mất mát
  - 55°C: ~50% mất mát
  - Tối đa: 70% mất mát

**Power Overhead (Tăng tiêu thụ điện):**
- **20-25°C**: Không tăng (0%)
- **Dưới 20°C**: Cần sưởi ấm, tăng ~2% mỗi 5°C dưới 20°C
- **Trên 25°C**: Cần làm mát, tăng theo hàm mũ:
  - 30°C: ~5% tăng
  - 40°C: ~15% tăng
  - 50°C: ~40% tăng
  - Tối đa: 50% tăng

### 3. CSV Output

File CSV giờ bao gồm 3 cột mới:
- `Average Temperature (°C)`: Nhiệt độ trung bình
- `Average Performance Loss`: Mất mát hiệu suất trung bình
- `Average Power Overhead (%)`: Tăng tiêu thụ điện trung bình

## Cách sử dụng

### Chạy thực nghiệm

```bash
cd emcontroller_demo/auto-schedule/experiments/usable-accept-rate
go run executor.go
```

Thực nghiệm sẽ:
1. Chạy với các số lượng app: 40, 60, 80
2. Lặp lại 20 lần cho mỗi số lượng app
3. So sánh các thuật toán: CompRand, BERand, Amaga, Ampga, Diktyoga, Mcssga
4. Tự động lấy nhiệt độ từ clouds và tính hao hụt

### Kết quả

File CSV được tạo: `usable_acceptance_rate_<appCount>.csv`

Ví dụ: `usable_acceptance_rate_60.csv` sẽ có format:
```
Algorithm Name,Maximum Scheduling Time (s),...,Average Temperature (°C),Average Performance Loss,Average Power Overhead (%)
CompRand,0.0176456,...,22.50,0.0125,1.25
BERand,0.0238954,...,22.50,0.0125,1.25
...
```

## Kịch bản nghiên cứu

### Kịch bản 1: Nhiệt độ cao
- Khi nhiệt độ trung bình > 35°C
- Performance loss tăng đáng kể (>10%)
- Power overhead tăng (>5%)
- Có thể ảnh hưởng đến acceptance rate do performance degradation

### Kịch bản 2: Nhiệt độ thấp
- Khi nhiệt độ trung bình < 20°C
- Cần sưởi ấm, tăng power overhead
- Performance loss nhẹ (<5%)

### Kịch bản 3: Nhiệt độ tối ưu
- Khi nhiệt độ trung bình 20-25°C
- Không có performance loss
- Không có power overhead
- Điều kiện lý tưởng cho scheduling

## Lưu ý

1. **Nhiệt độ được lấy từ Weather API**: Hệ thống tự động lấy nhiệt độ từ API dựa trên location của cloud (được cấu hình trong `cloud.go`)

2. **Tính trung bình**: Nhiệt độ và hao hụt được tính trung bình qua tất cả các lần scheduling thành công (usable solutions)

3. **Cloud location**: Location của cloud được định nghĩa trong hàm `getCloudLocation()` trong `cloud.go`. Có thể mở rộng bằng cách thêm mapping mới.

4. **Mô hình hao hụt**: Mô hình tính toán hao hụt có thể được điều chỉnh trong hàm `calculateLosses()` nếu cần.

## Mở rộng trong tương lai

1. **Temperature per cloud**: Thay vì trung bình, có thể track nhiệt độ của từng cloud riêng
2. **Temperature-aware scheduling**: Thuật toán scheduling có thể xem xét nhiệt độ khi quyết định
3. **Historical temperature**: Lưu lịch sử nhiệt độ để phân tích xu hướng
4. **Cost analysis**: Tính toán chi phí dựa trên power overhead

