package repository

import (
	"exoplanets/internal/app/ds"

	"gorm.io/gorm"
)

func (r *Repository) AddUser(user *ds.User) error {
	err := r.db.Create(user).Error

	if err != nil {
		return err
	}

	return nil
}
func (r *Repository) GetUserByLogin(login string) (*ds.User, error) {
	var user ds.User

	err := r.db.Where("login = ?", login).First(&user).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *Repository) GetUserByID(id uint) (*ds.User, error) {
	var user ds.User

	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}

	return &user, nil
}

var _ = gorm.ErrRecordNotFound
