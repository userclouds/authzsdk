package pagination

// ResponseFields represents pagination-specific fields present in every response.
type ResponseFields struct {
	HasNext bool   `json:"has_next"`
	Next    Cursor `json:"next,omitempty"`
	HasPrev bool   `json:"has_prev"`
	Prev    Cursor `json:"prev,omitempty"`
}
