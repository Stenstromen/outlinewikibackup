package types

type ExportResponse struct {
	Success bool `json:"success"`
	Data    struct {
		FileOperation struct {
			ID     string `json:"id"`
			State  string `json:"state"`
			Name   string `json:"name"`
			Format string `json:"format"`
		} `json:"fileOperation"`
	} `json:"data"`
	Status int  `json:"status"`
	Ok     bool `json:"ok"`
}

type ProgressResponse struct {
	Data struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Format string `json:"format"`
		Name   string `json:"name"`
	} `json:"data"`
	Status int  `json:"status"`
	Ok     bool `json:"ok"`
}
