package models

import (
	"gorm.io/gorm"
)

type Comment struct {
	gorm.Model
	Content string `gorm:"not null"`
	PostID  uint
	UserID  uint
	Post    Post
	User    User
}
