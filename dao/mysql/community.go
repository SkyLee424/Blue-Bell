package mysql

import (
	"bluebell/models"

	"github.com/pkg/errors"
)

func SelectCommunityList() ([]models.CommunityDTO, error) {
	var communityList []models.CommunityDTO
	res := db.Model(&models.Community{}).
		Select("community_id", "community_name").
		Find(&communityList)

	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get community list")
	}

	return communityList, nil
}

func SelectCommunityDetailByID(id int64) (*models.CommunityDTO, error) {
	detail := new(models.CommunityDTO)
	res := db.Model(&models.Community{}).First(&detail, "community_id = ?", id)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get community detail")
	}

	return detail, nil
}

func SelectCommunityIDs() ([]string, error) {
	var communityIDs []string
	res := db.Model(&models.Community{}).Select("community_id").Find(&communityIDs)
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "get communtiy ids")
	}
	return communityIDs, nil
}