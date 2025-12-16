### Chương trình này dùng để làm gì?
Trong Thuật toán Di truyền (Genetic Algorithm), chúng ta có thể đặt các **xác suất lai ghép (crossover probability)** và **xác suất đột biến (mutation probability)** khác nhau. Chương trình này thực hiện các kiểm thử để tìm **xác suất lai ghép** và **xác suất đột biến** tốt nhất.

### Cách sử dụng chương trình?

1. Comment các log không cần thiết để tránh quá nhiều log trong file output.
2. Trong thư mục này, chạy `go build`.
3. Sao chép (`scp`) file binary `optimizecpmp` đã tạo ra vào một VM.
4. Trên VM thực thi `nohup ./optimizecpmp > output.log 2>&1 &` để chạy chương trình ở chế độ nền.
5. Sau khi chương trình hoàn thành, dữ liệu sẽ nằm trong file `output.log`. 