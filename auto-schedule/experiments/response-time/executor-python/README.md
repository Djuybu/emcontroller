## Chương trình này dùng để làm gì?
Đây là công cụ để gọi các ứng dụng thí nghiệm, ghi lại dữ liệu liên quan đến thời gian phản hồi, và vẽ biểu đồ với dữ liệu đó.

Đây là executor chính của các thí nghiệm.

## Cách thực thi thí nghiệm sử dụng chương trình này?

### Bước 1: Quyết định các tham số

Cần quyết định:
- **repeat count**: Số lần lặp lại thí nghiệm
- **device count**: Số thiết bị gửi request
- **app count**: Số lượng ứng dụng
- **request count per app**: Số lần request mỗi ứng dụng

Sau khi quyết định, đặt các biến `REPEAT_COUNT`, `DEVICE_COUNT`, `APP_COUNT`, `REQ_COUNT_PER_APP` trong file `charts_drawer.py`.

#### repeat count (Số lần lặp lại)
Nên lặp lại thí nghiệm nhiều lần để giảm tác động của các yếu tố ngẫu nhiên.

Nếu cần lặp lại thí nghiệm 5 lần, tạo 5 thư mục có tên `repeat1`, `repeat2`, ..., `repeat5` trong thư mục `data`.

Trong mỗi thư mục `repeatX`, cần tạo:
- Các thư mục với tên của các thuật toán cần so sánh trong thí nghiệm.
- File `request_applications.json` để lưu trữ JSON body yêu cầu triển khai ứng dụng. (Vì trong mỗi lần lặp, nên sử dụng cùng các ứng dụng để so sánh các thuật toán khác nhau.)

#### device count, app count, request count per app
- `app count`: Số lượng ứng dụng trong yêu cầu triển khai/lập lịch. Khi sử dụng `auto-schedule/experiments/applications-generator` để tạo yêu cầu triển khai, cần đặt giá trị này.

- `device count` và `request count per app`: Trong mỗi lần lặp, có thể sử dụng nhiều thiết bị để yêu cầu các ứng dụng nhằm mô phỏng môi trường production thực tế, và mỗi thiết bị có thể truy cập mỗi app nhiều lần để giảm tác động của các yếu tố ngẫu nhiên.

### Bước 2: Tạo JSON body yêu cầu triển khai ứng dụng
Sử dụng `auto-schedule/experiments/applications-generator` để tạo yêu cầu triển khai. 

Nếu `repeat count` là 3, tạo 3 JSON body và đặt chúng trong `data/repeatX/request_applications.json`.

### Bước 3: Triển khai ứng dụng
Gửi yêu cầu đến multi-cloud manager để lập lịch và triển khai ứng dụng:
- Sử dụng JSON body trong `data/repeatX/request_applications.json`
- Sử dụng HTTP header `Mcm-Scheduling-Algorithm` để đặt tên thuật toán

**Ví dụ:**
```bash
curl -i -X POST \
  -H "Content-Type:application/json" \
  -H "Mcm-Scheduling-Algorithm:mcssga" \
  -d @data/repeat1/request_applications.json \
  http://localhost:20000/doNewAppGroup
```

### Bước 4: Gọi ứng dụng
Sau khi ứng dụng được triển khai và đang chạy:

1. Chạy `python3.11 -u caller.py` `request count per app` lần để truy cập các ứng dụng
2. Chương trình sẽ tạo `request count per app` file CSV trong thư mục `data`
3. Di chuyển tất cả các file CSV đã tạo vào thư mục tương ứng `data/repeatX/{algorithm name}`

**Ví dụ:**
```bash
# Gọi ứng dụng 10 lần
for i in {1..10}; do
  python3.11 -u caller.py
done

# Di chuyển kết quả
mv data/*.csv data/repeat1/mcssga/
```

### Bước 5: Vẽ biểu đồ
Trên hệ thống có GUI:

**Vẽ biểu đồ CDF:**
```bash
python -u charts_drawer.py
```

**Vẽ biểu đồ điểm:**
```bash
python -u charts_drawer_no_cdf.py
```

## Cấu trúc thư mục
```
data/
├── repeat1/
│   ├── mcssga/           # Kết quả thuật toán MCSSGA
│   │   ├── result_1.csv
│   │   ├── result_2.csv
│   │   └── ...
│   ├── amaga/            # Kết quả thuật toán AMAGA
│   ├── ampga/            # Kết quả thuật toán AMPGA
│   └── request_applications.json
├── repeat2/
│   └── ...
└── repeat3/
    └── ...
```

## Lưu ý
- Đảm bảo các ứng dụng đã được triển khai và đang chạy trước khi gọi
- Kiểm tra kết nối mạng đến các ứng dụng
- File CSV chứa thông tin: thời gian phản hồi, nhiệt độ, performance loss, power overhead