package main

// Datenstrukturen für Templates und APIs
type Project struct {
	ID         int64    `json:"id"`
	Folder     string   `json:"folder"`
	Title      string   `json:"title"`
	Date       string   `json:"date"`
	Desc       string   `json:"desc"`
	MainImg    string   `json:"main_img"`
	GalleryStr string   `json:"gallery_str"`
	GalleryArr []string `json:"gallery_arr"`
}

type ChatRequest struct {
	Message string `json:"message"`
}

type ChatResponse struct {
	Response string `json:"response"`
}

type ChatLog struct {
	ID              int    `json:"id"`
	UserIP          string `json:"user_ip"`
	OriginalMessage string `json:"original_message"`
	MaskedMessage   string `json:"masked_message"`
}
