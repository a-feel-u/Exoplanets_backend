package ds

type User struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Login    string `gorm:"type:varchar(40);uniqueIndex;not null" json:"login"`
	Password string `gorm:"type:varchar(255);not null" json:"-"`
}
