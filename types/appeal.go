package types

import (
	"gorm.io/plugin/soft_delete"
)

type ModelAppeal struct {
	Id uint64 `json:"id,omitempty" gorm:"primaryKey;autoIncrement:true;column:id;not null"`

	CreatedAt uint32                `json:"created_at,omitempty" gorm:"autoCreateTime;<-:create;column:created_at;not null"`
	UpdatedAt uint32                `json:"updated_at,omitempty" gorm:"autoUpdateTime;<-;column:updated_at;not null"`
	DeletedAt soft_delete.DeletedAt `json:"deleted_at,omitempty" gorm:"column:deleted_at;not null"`

	PaperId      uint64 `json:"paper_id,omitempty" gorm:"column:paper_id;not null"`
	AppealStatus string `json:"appeal_status,omitempty" gorm:"column:appeal_status;not null"`
	AppealInfo   string `json:"appeal_info,omitempty" gorm:"column:appeal_info"`
	ReviewInfo   string `json:"review_info,omitempty" gorm:"column:review_info"`
	AppealResult string `json:"appeal_result,omitempty" gorm:"column:appeal_result"`
}
