package domain

type UploadFileRequest struct {
	ChunkIndex int `json:"chunk_index" query:"chunk_index" form:"chunk_index"`
}

type MergeChunksRequest struct {
	TotalChunks int    `json:"total_chunks" query:"total_chunks"`
	FileName    string `json:"file_name" query:"file_name"`
}
