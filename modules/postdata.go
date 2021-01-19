package modules

// PostDatav2 ...
type PostDatav2 struct {
	Agent        string `json:"agent" validate:"required,min=4,max=8" binding:"required"`
	User         string `json:"user" validate:"required,alphanum,min=3,max=6" binding:"required"`
	RequestID    string `json:"requestid" validate:"required,uuid" binding:"required"`
	Amount       int    `json:"amount" validate:"required,numeric,min=1,max=999999999" binding:"required"`
	Token        string `json:"token" validate:"required,alphanum,min=1,max=32" binding:"required"`
	Action       string `json:"action" validate:"required,alpha,min=4,max=8" binding:"required"`
	ClientIP     string
	RequestProto string
	RequestURL   string
	Detail       interface{} `json:"detail"`
}

// PostDeductv2 ...
type PostDeductv2 struct {
	GameID int `json:"gameid" validate:"required,numeric,min=1,max=99999" binding:"required"`
}

// PostStorev2 ...
type PostStorev2 struct{}

// PostData from request json
type PostData struct {
	Agent     string `json:"agent" validate:"required" binding:"required"`
	User      string `json:"user" validate:"required" binding:"required"`
	RequestID string `json:"requestid" validate:"required" binding:"required"`
	Amount    int    `json:"amount" validate:"required" binding:"required"`
	Action    string `json:"action" validate:"required" binding:"required"`
	Token     string `json:"token" validate:"required" binding:"required"`
}

// PostDeduct ...
type PostDeduct struct {
	PostData
	GameID int `json:"gameid" binding:"required"`
}

// PostSave ...
type PostSave struct {
	PostData
}
