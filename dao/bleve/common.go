package bleve

import (
	"github.com/blevesearch/bleve/v2"
	"github.com/pkg/errors"
)

var postIndex bleve.Index

func InitBleve()  {
	var err error
	// 创建 post 的索引
	postIndex, err = createIndex("bluebell_post.bleve")
	if err != nil {
		panic(err.Error())
	}
}

func GetPostIndex() bleve.Index {
	return postIndex
}

func matchQuerySearch(index bleve.Index, match string, size int, from int, useTime bool) (*bleve.SearchResult, error) {
	query := bleve.NewMatchQuery(match)
	search := bleve.NewSearchRequestOptions(query, size, from, false)
	if useTime {
		search.SortBy([]string{"-created_time"}) // 按 created_time 降序排序
	}
	searchResults, err := index.Search(search)
	return searchResults, errors.Wrap(err, "bleve:matchQuerySearch: Search")
}

func createIndex(path string) (bleve.Index, error) {
	// 定义映射
	indexMapping := bleve.NewIndexMapping()
	textFieldMapping := bleve.NewTextFieldMapping()
	textFieldMapping.Analyzer = "cjk" // 使用中文分词器
	indexMapping.AddCustomAnalyzer("cjk", map[string]interface{}{
		"type":   "cjk",
		"locale": "zh",
	})
	// 为创建时间创建索引，以实现按照时间排序
	indexMapping.DefaultMapping.AddFieldMappingsAt("created_time", bleve.NewDateTimeFieldMapping())

	// 打开或创建索引
	index, err := bleve.Open(path)
	if err == bleve.ErrorIndexMetaMissing || err == bleve.ErrorIndexPathDoesNotExist {
		index, err = bleve.New(path, indexMapping)
	}
	return index, err
}
