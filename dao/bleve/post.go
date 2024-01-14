package bleve

import (
	"bluebell/models"
	"strconv"

	"github.com/pkg/errors"
)

func CreatePost(doc *models.PostDoc) error {
	return errors.Wrap(postIndex.Index(strconv.Itoa(int(doc.PostID)), doc), "bleve:CreatePost: Index")
}

func GetPostIDsByKeyword(params *models.ParamPostListByKeyword) ([]string, int, error) {
	from := (params.PageNum - 1) * params.PageSize
	orderByTime := params.OrderBy == "time"

	searchResults, err := matchQuerySearch(postIndex, params.Keyword, int(params.PageSize), int(from), orderByTime)
	if err != nil {
		return nil, 0, errors.Wrap(err, "bleve:GetPostIDsByKeyword: matchQuerySearch")
	}

	postIDs := make([]string, 0, len(searchResults.Hits))
	for _, res := range searchResults.Hits {
		postIDs = append(postIDs, res.ID)
	}

	return postIDs, int(searchResults.Total), nil
}

func UpdatePost(doc *models.PostDoc) error {
	err := DeletePost(doc.PostID) // 先删除原有 doc
	if err != nil {
		return errors.Wrap(err, "bleve:UpdatePost: DeletePost")
	}
	// 再创建
	return errors.Wrap(CreatePost(doc), "bleve:UpdatePost: CreatePost")
}

func DeletePost(postID int64) error {
	return errors.Wrap(postIndex.Delete(strconv.Itoa(int(postID))), "bleve:DeletePost: Delete")
}
