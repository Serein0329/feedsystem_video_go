package account

type Account struct {
	ID           uint   `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"unique" json:"username"`
	Password     string `json:"-"`
	Token        string `json:"-"`
	RefreshToken string `json:"-"`
	AvatarURL    string `gorm:"type:varchar(512)" json:"avatar_url,omitempty"`
	Bio          string `gorm:"type:varchar(255)" json:"bio,omitempty"`
}

type CreateAccountRequest struct {
	Username string `json:"username" binding:"required,max=20"`
	Password string `json:"password" binding:"required,min=8,max=20"`
}

type RenameRequest struct {
	NewUsername string `json:"new_username" binding:"required,max=20"`
}

type FindByIDRequest struct {
	ID uint `json:"id" binding:"required"`
}

type FindByUsernameRequest struct {
	Username string `json:"username" binding:"required,max=20"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8,max=20"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=20"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,max=20"`
	Password string `json:"password" binding:"required,min=8,max=20"`
}

type UpdateBioRequest struct {
	Bio string `json:"bio" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type GetProfileRequest struct {
	AccountID uint `json:"account_id" binding:"required"`
}

type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

type RenameResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}
type GetProfileResponse struct {
	Account
	VideoCount    int64 `json:"video_count"`
	TotalLikes    int64 `json:"total_likes"`
	FollowerCount int64 `json:"follower_count"`
	VloggerCount  int64 `json:"vlogger_count"`
}
