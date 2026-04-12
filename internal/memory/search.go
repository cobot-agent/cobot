package memory

import (
	"context"
	"path/filepath"
	"time"

	"github.com/blevesearch/bleve/v2"

	cobot "github.com/cobot-agent/cobot/pkg"
)

type drawerDoc struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	WingID    string    `json:"wing_id"`
	RoomID    string    `json:"room_id"`
	HallType  string    `json:"hall_type"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) openIndex() (bleve.Index, error) {
	path := filepath.Join(s.bleveDir, "drawers.bleve")
	idx, err := bleve.Open(path)
	if err == nil {
		return idx, nil
	}
	mapping := bleve.NewIndexMapping()
	docMapping := bleve.NewDocumentMapping()

	contentField := bleve.NewTextFieldMapping()
	contentField.Analyzer = "en"
	docMapping.AddFieldMappingsAt("content", contentField)

	wingField := bleve.NewKeywordFieldMapping()
	docMapping.AddFieldMappingsAt("wing_id", wingField)

	roomField := bleve.NewKeywordFieldMapping()
	docMapping.AddFieldMappingsAt("room_id", roomField)

	hallField := bleve.NewKeywordFieldMapping()
	docMapping.AddFieldMappingsAt("hall_type", hallField)

	dateField := bleve.NewDateTimeFieldMapping()
	docMapping.AddFieldMappingsAt("created_at", dateField)

	mapping.AddDocumentMapping("drawer", docMapping)
	mapping.DefaultMapping = docMapping

	return bleve.New(path, mapping)
}

func (s *Store) indexDrawer(ctx context.Context, d *drawerDoc) error {
	return s.bleveIdx.Index(d.ID, d)
}

func (s *Store) searchDrawers(ctx context.Context, query *cobot.SearchQuery) ([]*cobot.SearchResult, error) {
	if query.Text == "" && query.WingID == "" && query.RoomID == "" && query.HallType == "" {
		req := bleve.NewSearchRequest(bleve.NewMatchAllQuery())
		req.Fields = []string{"content", "wing_id", "room_id"}
		if query.Limit > 0 {
			req.Size = query.Limit
		} else {
			req.Size = 10
		}
		return s.executeSearch(s.bleveIdx, req)
	}

	bq := bleve.NewBooleanQuery()

	if query.Text != "" {
		mq := bleve.NewMatchQuery(query.Text)
		mq.SetField("content")
		bq.AddMust(mq)
	}

	if query.WingID != "" {
		wq := bleve.NewTermQuery(query.WingID)
		wq.SetField("wing_id")
		bq.AddMust(wq)
	}
	if query.RoomID != "" {
		rq := bleve.NewTermQuery(query.RoomID)
		rq.SetField("room_id")
		bq.AddMust(rq)
	}
	if query.HallType != "" {
		hq := bleve.NewTermQuery(query.HallType)
		hq.SetField("hall_type")
		bq.AddMust(hq)
	}

	req := bleve.NewSearchRequest(bq)
	req.Fields = []string{"content", "wing_id", "room_id"}
	if query.Limit > 0 {
		req.Size = query.Limit
	} else {
		req.Size = 10
	}
	return s.executeSearch(s.bleveIdx, req)
}

func (s *Store) executeSearch(idx bleve.Index, req *bleve.SearchRequest) ([]*cobot.SearchResult, error) {
	resp, err := idx.Search(req)
	if err != nil {
		return nil, err
	}
	var results []*cobot.SearchResult
	for _, hit := range resp.Hits {
		sr := &cobot.SearchResult{
			DrawerID: hit.ID,
			Score:    hit.Score,
		}
		if v, ok := hit.Fields["content"]; ok {
			sr.Content, _ = v.(string)
		}
		if v, ok := hit.Fields["wing_id"]; ok {
			sr.WingID, _ = v.(string)
		}
		if v, ok := hit.Fields["room_id"]; ok {
			sr.RoomID, _ = v.(string)
		}
		results = append(results, sr)
	}
	return results, nil
}
