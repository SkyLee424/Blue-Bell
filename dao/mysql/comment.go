package mysql

import (
	"bluebell/models"
	"fmt"

	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

func CreateCommentContent(tx *gorm.DB, content *models.CommentContent) error {
	useDB := getUseDB(tx)

	res := useDB.Create(&content)
	return errors.Wrap(res.Error, "mysql: CreateCommentContent failed")
}

func CreateCommentSubject(tx *gorm.DB, ID, objID int64, objType int8) error {
	useDB := getUseDB(tx)

	commentSubject := &models.CommentSubject{
		ID:        ID,
		ObjID:     objID,
		ObjType:   objType,
		Count:     0,
		RootCount: 0,
	}

	res := useDB.Create(&commentSubject)
	return errors.Wrap(res.Error, "mysql: CreateCommentSubject failed")
}

func CreateCommentLikeOrHateUser(tx *gorm.DB, commentID, userID, objID int64, objType int8, like bool) error {
	useDB := getUseDB(tx)
	var res *gorm.DB

	// 在添加之前，检查有没有重复项（cid、uid、oid、otype 均相同）
	exist, err := CheckCidUidIfExist(useDB, commentID, userID, like)
	if err != nil {
		return errors.Wrap(err, "mysql:CreateCommentLikeOrHateUser: CheckCidUidIfExist")
	}
	if exist {
		return nil
	}

	// 没有重复项，可以添加
	if like {
		res = useDB.Create(&models.CommentUserLikeMapping{
			CommentID: commentID,
			UserID:    userID,
			ObjID:     objID,
			ObjType:   objType,
		})
	} else {
		res = useDB.Create(&models.CommentUserHateMapping{
			CommentID: commentID,
			UserID:    userID,
			ObjID:     objID,
			ObjType:   objType,
		})
	}

	// 忽略重复 key 的错误
	if res.Error != nil {
		rawErr, _ := res.Error.(*mysql.MySQLError)
		if rawErr.Number != 1062 {
			return errors.Wrap(res.Error, "mysql:CreateCommentLikeOrHateUser: Create")
		}
	}

	return nil
}

func CreateCommentIndex(tx *gorm.DB, index *models.CommentIndex) error {
	useDB := getUseDB(tx)

	res := useDB.Create(&index)
	return errors.Wrap(res.Error, "mysql: CreateCommentIndex failed")
}

// 递增 subject 的 count
func IncrCommentSubjectCountField(tx *gorm.DB, field string, objID int64, objType int8, offset int) error {
	if offset == 0 {
		return nil
	}
	useDB := getUseDB(tx)
	expr := fmt.Sprintf("%s + %d", field, offset)
	res := useDB.Model(&models.CommentSubject{}).Where("obj_id = ? AND obj_type = ?", objID, objType).Update(field, gorm.Expr(expr))
	return errors.Wrap(res.Error, "mysql: IncrCommentSubjectField failed")
}

// 递增根评论的 count
func IncrCommentIndexCountField(tx *gorm.DB, field string, id int64, offset int) error {
	if offset == 0 {
		return nil
	}
	useDB := getUseDB(tx)
	expr := fmt.Sprintf("%s + %d", field, offset)
	res := useDB.Model(&models.CommentIndex{}).Where("id = ?", id).Update(field, gorm.Expr(expr))
	return errors.Wrap(res.Error, "mysql: IncrCommentIndexCount failed")
}

func SelectCommentSubjectCountField(tx *gorm.DB, field string, objID int64, objType int8) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentSubject{}).Select(field).Where("obj_id = ? AND obj_type = ?", objID, objType).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentSubjectCountField failed")
}

func SelectCommentIndexCountField(tx *gorm.DB, field string, root int64) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentIndex{}).Select(field).Where("id = ?", root).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentIndexCountField")
}

// 根据 obj_id、obj_type 检查某个 subject 是否存在
func SelectCommentSubjectCount(tx *gorm.DB, objID int64, objType int8) (int, error) {
	useDB := getUseDB(tx)
	var count int
	res := useDB.Model(&models.CommentSubject{}).Select("count(*)").Where("obj_id = ? AND obj_type = ?", objID, objType).Scan(&count)
	return count, errors.Wrap(res.Error, "mysql: SelectCommentSubjectCount")
}

// 获取根评论 ID，不用排序也可以保证按楼层升序排列
func SelectRootCommentIDs(tx *gorm.DB, objType int8, objID int64) ([]int64, error) {
	useDB := getUseDB(tx)
	rootCommentIDs := make([]int64, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("id").Where("root = 0 and obj_type = ? and obj_id = ?", objType, objID).
		Scan(&rootCommentIDs)
	return rootCommentIDs, errors.Wrap(res.Error, "mysql: SelectCommentIDs")
}

func SelectSubCommentIDs(tx *gorm.DB, root int64) ([]int64, error) {
	useDB := getUseDB(tx)
	subCommentIDs := make([]int64, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("id").Where("root = ?", root).
		Scan(&subCommentIDs)
	return subCommentIDs, errors.Wrap(res.Error, "mysql: SelectSubCommentIDs")
}

func SelectFloorsByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]int, error) {
	useDB := getUseDB(tx)
	floors := make([]int, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("floor").Where("id in ?", commentIDs).
		Scan(&floors)
	return floors, errors.Wrap(res.Error, "mysql: SelectFloorsByCommentIDs")
}

// 根据传入的评论 ID，查 comment 的元数据（不包括 content）
func SelectCommentMetaDataByCommentIDs(tx *gorm.DB, field string, commentIDs []int64) ([]models.CommentDTO, error) {
	useDB := getUseDB(tx)

	tmp := make([]models.CommentIndexDTO, 0)

	sql := fmt.Sprintf("select c.*, user_name, avatar from comment_indices c join users u on c.user_id = u.user_id where c.%s in ?", field)
	res := useDB.Raw(sql, commentIDs).Scan(&tmp)

	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "mysql: SelectCommentMetaDataByCommentIDs")
	}

	return selectCommentContentHelper(tmp), nil
}

func SelectCommentContentByCommentIDs(tx *gorm.DB, commentIDs []int64) ([]models.CommentContentDTO, error) {
	useDB := getUseDB(tx)

	contents := make([]models.CommentContentDTO, 0)

	res := useDB.Model(&models.CommentContent{}).Where("comment_id in ?", commentIDs).Scan(&contents)

	return contents, errors.Wrap(res.Error, "mysql: SelectCommentContentByCommentIDs")
}

func selectCommentContentHelper(tmp []models.CommentIndexDTO) []models.CommentDTO {
	comments := make([]models.CommentDTO, 0, len(tmp))
	for i := 0; i < len(tmp); i++ {
		comments = append(comments, models.CommentDTO{
			CommentID: tmp[i].ID,
			ObjID:     tmp[i].ObjID,
			Type:      tmp[i].ObjType,
			Root:      tmp[i].Root,
			Parent:    tmp[i].Parent,
			UserID:    tmp[i].UserID,
			UserName:  tmp[i].UserName,
			Avatar:    tmp[i].Avatar,
			Floor:     tmp[i].Floor,
			Like:      tmp[i].Like,
			CreatedAt: tmp[i].CreatedAt,
			UpdatedAt: tmp[i].UpdatedAt,
		})
	}
	return comments
}

func SelectUserIDByCommentID(tx *gorm.DB, commentID int64) (int64, error) {
	useDB := getUseDB(tx)

	var userID int64
	res := useDB.Model(&models.CommentIndex{}).Select("user_id").Where("id = ?", commentID).Scan(&userID)

	return userID, errors.Wrap(res.Error, "mysql: SelectUserIDByCommentID")
}

func SelectCommentIDsByObjID(tx *gorm.DB, objID int64, objType int8) ([]int64, error) {
	useDB := getUseDB(tx)

	commentIDs := make([]int64, 0)
	res := useDB.Model(&models.CommentIndex{}).Select("id").Where("obj_id = ? AND obj_type = ?", objID, objType).Scan(&commentIDs)

	return commentIDs, errors.Wrap(res.Error, "mysql:SelectCommentIDsByObjID")
}

func CheckIsRootComment(tx *gorm.DB, commentID int64) (bool, error) {
	useDB := getUseDB(tx)

	var root int64
	res := useDB.Model(&models.CommentIndex{}).Select("root").Where("id = ?", commentID).Scan(&root)

	return root == 0, errors.Wrap(res.Error, "mysql: CheckIsRootComment")
}

func CheckCidUidIfExist(tx *gorm.DB, commentID, userID int64, like bool) (bool, error) {
	useDB := getUseDB(tx)
	var res *gorm.DB
	var count int
	if like {
		res = useDB.Model(&models.CommentUserLikeMapping{}).Select("count(*)").Where("comment_id = ? and user_id = ?", commentID, userID).Scan(&count)
	} else {
		res = useDB.Model(&models.CommentUserHateMapping{}).Select("count(*)").Where("comment_id = ? and user_id = ?", commentID, userID).Scan(&count)
	}
	return count == 1, errors.Wrap(res.Error, "mysql: CheckCidUidIfExist")
}

func SelectSubCommentIDsByField(tx *gorm.DB, commentID int64, field string) ([]int64, error) {
	useDB := getUseDB(tx)
	subCommentIDs := make([]int64, 0)

	res := useDB.Model(&models.CommentIndex{}).Select("id").Where(gorm.Expr(field+" = ?", commentID)).Scan(&subCommentIDs)

	return subCommentIDs, errors.Wrap(res.Error, "mysql: SelectSubCommentIDsByField")
}

func SelectCommentRootIDByCommentID(tx *gorm.DB, commentID int64) (int64, error) {
	useDB := getUseDB(tx)
	var root int64

	res := useDB.Model(&models.CommentIndex{}).Select("root").Where("id = ?", commentID).Scan(&root)
	return root, errors.Wrap(res.Error, "mysql: SelectCommentRootIDByCommentID")
}

func DeleteCommentIndexByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentIndex{}, commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentIndexByCommentIDs")
}

func DeleteCommentContentByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentContent{}, "comment_id in ?", commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentContentByCommentIDs")
}

func DeleteCommentUserLikeMappingByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentUserLikeMapping{}, "comment_id in ?", commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentUserLikeMappingByCommentIDs")
}

func DeleteCommentUserHateMappingByCommentIDs(tx *gorm.DB, commentIDs []int64) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentUserHateMapping{}, "comment_id in ?", commentIDs)
	return errors.Wrap(res.Error, "mysql: DeleteCommentUserHateMappingByCommentIDs")
}

func DeleteCommentSubjectByObjID(tx *gorm.DB, objID int64, objType int8) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentSubject{}, "obj_id = ? AND obj_type = ?", objID, objType)
	return errors.Wrap(res.Error, "mysql: DeleteCommentSubjectByObjID")
}

func DeleteCommentIndexByObjID(tx *gorm.DB, objID int64, objType int8) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentIndex{}, "obj_id = ? AND obj_type = ?", objID, objType)
	return errors.Wrap(res.Error, "mysql: DeleteCommentIndexByObjID")
}

func DeleteCommentUserLikeMappingByObjID(tx *gorm.DB, objID int64, objType int8) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentUserLikeMapping{}, "obj_id = ? AND obj_type = ?", objID, objType)
	return errors.Wrap(res.Error, "mysql: DeleteCommentUserLikeMappingByObjID")
}

func DeleteCommentUserHateMappingByObjID(tx *gorm.DB, objID int64, objType int8) error {
	useDB := getUseDB(tx)
	res := useDB.Delete(&models.CommentUserHateMapping{}, "obj_id = ? AND obj_type = ?", objID, objType)
	return errors.Wrap(res.Error, "mysql: DeleteCommentUserHateMappingByObjID")
}

func SelectCommentUserLikeOrHateList(tx *gorm.DB, userID, ObjID int64, ObjType int8, Like bool) ([]int64, error) {
	useDB := getUseDB(tx)
	var res *gorm.DB
	var commentIDs []int64
	if Like {
		res = useDB.Model(&models.CommentUserLikeMapping{}).Select("comment_id").Where("user_id = ? and obj_id = ? and obj_type = ?", userID, ObjID, ObjType).Scan(&commentIDs)
	} else {
		res = useDB.Model(&models.CommentUserHateMapping{}).Select("comment_id").Where("user_id = ? and obj_id = ? and obj_type = ?", userID, ObjID, ObjType).Scan(&commentIDs)
	}
	if res.Error != nil {
		return nil, errors.Wrap(res.Error, "mysql:SelectCommentUserLikeOrHateList")
	}
	return commentIDs, nil
}
