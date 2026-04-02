# Quiz App

Ứng dụng luyện thi trắc nghiệm cá nhân. Import bộ câu hỏi do AI tạo và luyện tập qua chế độ flashcard hoặc thi thử.

[English](README.md)

## Bắt đầu nhanh

```bash
# Build
go build -o quiz .

# Import câu hỏi
./quiz import questions.json

# Khởi động server
./quiz
# Mở http://localhost:8080
```

### Docker

```bash
docker compose up --build
# Mở http://localhost:8080
```

Dữ liệu được lưu trong Docker volume. Để import câu hỏi trong container:

```bash
docker compose exec quiz ./quiz import /data/questions.json
```

### Biến môi trường

| Biến | Mặc định | Mô tả |
|------|----------|-------|
| `PORT` | `8080` | Cổng server |
| `DB_PATH` | `quiz.db` | Đường dẫn file database SQLite |

## Cách sử dụng

### Import câu hỏi

**CLI:**

```bash
./quiz import questions.json
```

**Giao diện web:**

Truy cập `http://localhost:8080/import`, paste JSON vào textarea và submit.

Nếu chủ đề đã tồn tại, câu hỏi mới sẽ được thêm vào chủ đề đó.

### Chế độ luyện tập

**Flashcard** — Trả lời từng câu một, xem kết quả ngay lập tức, rồi chuyển sang câu tiếp. Câu hỏi được xáo trộn và không lặp lại trong cùng một phiên.

**Thi thử (Exam)** — Chọn số lượng câu hỏi, trả lời hết, nộp bài và xem điểm kèm phần review chi tiết từng câu đúng/sai.

### Thống kê

Xem tỷ lệ chính xác theo chủ đề, lịch sử làm bài và điểm cao nhất/trung bình tại `/stats`.

## Định dạng dữ liệu đầu vào

Câu hỏi được import dưới dạng JSON với cấu trúc sau:

```json
{
  "subject": "Tên chủ đề",
  "questions": [
    {
      "content": "Nội dung câu hỏi?",
      "explanation": "Giải thích tùy chọn, hiển thị khi xem kết quả",
      "answers": [
        {"label": "A", "content": "Đáp án thứ nhất", "is_correct": false},
        {"label": "B", "content": "Đáp án thứ hai", "is_correct": true},
        {"label": "C", "content": "Đáp án thứ ba", "is_correct": false},
        {"label": "D", "content": "Đáp án thứ tư", "is_correct": false}
      ]
    }
  ]
}
```

### Mô tả các trường

| Trường | Kiểu | Bắt buộc | Mô tả |
|--------|------|----------|-------|
| `subject` | string | có | Tên chủ đề. Nếu đã tồn tại, câu hỏi sẽ được thêm vào |
| `questions` | array | có | Danh sách câu hỏi |
| `questions[].content` | string | có | Nội dung câu hỏi |
| `questions[].explanation` | string | không | Giải thích hiển thị khi xem kết quả. Chỉ thêm khi đáp án không hiển nhiên hoặc cần làm rõ |
| `questions[].answers` | array | có | Danh sách đáp án (thường là 4) |
| `questions[].answers[].label` | string | có | Nhãn đáp án (ví dụ: "A", "B", "C", "D") |
| `questions[].answers[].content` | string | có | Nội dung đáp án |
| `questions[].answers[].is_correct` | boolean | có | `true` cho đáp án đúng, `false` cho đáp án sai |
| `questions[].multi_answer` | boolean | không | Đặt `true` cho câu có nhiều đáp án đúng. Tự động phát hiện nếu bỏ qua (dựa vào số đáp án `is_correct: true`) |

### Quy tắc

- Mỗi câu hỏi phải có **ít nhất một** đáp án với `is_correct: true`
- Với câu hỏi nhiều đáp án đúng, nhiều đáp án có thể có `is_correct: true` — ứng dụng sẽ hiển thị checkbox thay vì radio button
- `multi_answer` tự động phát hiện nếu bỏ qua: câu hỏi có 2+ đáp án đúng tự động được xem là nhiều đáp án
- Label phải duy nhất trong mỗi câu hỏi (A, B, C, D)
- Không giới hạn số đáp án mỗi câu, nhưng 4 là tiêu chuẩn

## Tạo câu hỏi bằng AI

Copy prompt bên dưới và paste vào bất kỳ LLM nào (ChatGPT, Claude, Gemini, v.v.). Thay thế các placeholder bằng chủ đề và số lượng mong muốn.

---

<pre>
Tạo bộ câu hỏi trắc nghiệm để ôn tập/luyện thi.

Chủ đề: [CHỦ ĐỀ CỦA BẠN]
Số lượng câu hỏi: [SỐ LƯỢNG]
Ngôn ngữ: [Tiếng Việt / English / ...]

Yêu cầu:
- Mỗi câu hỏi có đúng 4 đáp án (A, B, C, D)
- Phần lớn câu hỏi có đúng một đáp án đúng. Một số câu có thể có nhiều đáp án đúng — với những câu đó, đánh dấu tất cả đáp án đúng bằng "is_correct": true
- Câu hỏi nên đa dạng về độ khó (dễ, trung bình, khó)
- Bao phủ nhiều khía cạnh khác nhau của chủ đề
- Tránh câu hỏi đánh lừa; tập trung kiểm tra hiểu biết thực sự
- Chỉ thêm trường "explanation" khi đáp án không hiển nhiên, dễ nhầm lẫn, hoặc cần giải thích thêm. KHÔNG thêm explanation cho các câu hỏi đơn giản.

Chỉ trả về JSON hợp lệ theo đúng format dưới đây, không giải thích hay markdown:

{
  "subject": "[Tên chủ đề]",
  "questions": [
    {
      "content": "Nội dung câu hỏi?",
      "explanation": "Chỉ khi cần - giải thích tại sao đáp án đúng",
      "answers": [
        {"label": "A", "content": "Đáp án A", "is_correct": false},
        {"label": "B", "content": "Đáp án B", "is_correct": true},
        {"label": "C", "content": "Đáp án C", "is_correct": false},
        {"label": "D", "content": "Đáp án D", "is_correct": false}
      ]
    }
  ]
}
</pre>

---

**Ví dụ sử dụng:**

> Tạo bộ câu hỏi trắc nghiệm để ôn tập/luyện thi.
>
> Chủ đề: AWS Solutions Architect Associate - S3 & Storage Services
> Số lượng câu hỏi: 20
> Ngôn ngữ: Tiếng Việt

Sau khi LLM trả về JSON, bạn có thể:
1. Lưu vào file và chạy `./quiz import file.json`
2. Paste trực tiếp vào form import trên web tại `/import`
