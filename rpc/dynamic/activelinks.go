package dynamic

import (
	"encoding/json"
	"strings"

	util "github.com/duality-solutions/web-bridge/internal/utilities"
)

// ActiveLinks stores the completed link list returned by
type ActiveLinks struct {
	Links       []Link `json:"link"`
	LockedLinks int    `json:"locked_links"`
}

func newActiveLinks() ActiveLinks {
	var links ActiveLinks
	links.Links = []Link{}
	links.LockedLinks = 0
	return links
}

// GetActiveLinks returns all the active links
func (d *Dynamicd) GetActiveLinks() (*ActiveLinks, error) {
	var linksGeneric map[string]interface{}
	var links ActiveLinks = newActiveLinks()
	req, _ := NewRequest("dynamic-cli link complete")
	rawResp := []byte(<-d.ExecCmdRequest(req))
	errUnmarshal := json.Unmarshal(rawResp, &linksGeneric)
	if errUnmarshal != nil {
		return &links, errUnmarshal
	}
	for k, v := range linksGeneric {
		if strings.HasPrefix(k, "link-") {
			b, err := json.Marshal(v)
			if err == nil {
				var link Link
				errUnmarshal = json.Unmarshal(b, &link)
				if errUnmarshal != nil {
					util.Error.Println("Inner error", errUnmarshal)
					return nil, errUnmarshal
				}

				links.Links = append(links.Links, link)
			}
		}
	}
	return &links, nil
}
