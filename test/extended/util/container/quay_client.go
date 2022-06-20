package container

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type AuthInfo struct {
	Authorization string `json:"authorization"`
}

// TagInfo
type TagInfo struct {
	Name             string `json:"name"`
	Reversion        bool   `json:"reversion"`
	Start_ts         int64  `json:"start_ts"`
	End_ts           int64  `json:"end_ts"`
	Image_Id         string `json:"image_id"`
	Last_modified    string `json:"last_modified"`
	Expiration       string `json:"expiration"`
	manifest_digest  string `json:"manifest_digest"`
	Docker_image_id  string `json:"docker_image_id"`
	Is_manifest_list bool   `json:"is_manifest_list"`
	Size             int64  `json:"size"`
}

type TagsResult struct {
	has_additional bool      `json:"has_additional"`
	page           int       `json:"page"`
	Tags           []TagInfo `json:"tags"`
}

// PodmanCLI provides function to run the docker command
type QuayCLI struct {
	EndPointPre   string
	Authorization string
}

func NewQuayCLI() *QuayCLI {
	newclient := &QuayCLI{}
	newclient.EndPointPre = "https://quay.io/api/v1/repository/"
	authString := ""
	authFilepath := ""
	if strings.Compare(os.Getenv("QUAY_AUTH_FILE"), "") != 0 {
		authFilepath = os.Getenv("QUAY_AUTH_FILE")
	} else {
		authFilepath = "/home/cloud-user/.docker/auto/quay_auth.json"
	}
	if _, err := os.Stat(authFilepath); os.IsNotExist(err) {
		e2e.Logf("Quay auth file does not exist")
	} else {
		content, err := ioutil.ReadFile(authFilepath)
		if err != nil {
			e2e.Logf("File reading error")
		} else {
			var authJson AuthInfo
			if err := json.Unmarshal(content, &authJson); err != nil {
				e2e.Logf("parser json error")
			} else {
				authString = "Bearer " + authJson.Authorization
			}
		}
	}
	if strings.Compare(os.Getenv("QUAY_AUTH"), "") != 0 {
		e2e.Logf("get quay auth from env QUAY_AUTH")
		authString = "Bearer " + os.Getenv("QUAY_AUTH")
	}
	if strings.Compare(authString, "Bearer ") == 0 {
		e2e.Failf("get quay auth failed!")
	}
	newclient.Authorization = authString
	return newclient
}

// TryDeleteTag will delete the image
func (c *QuayCLI) TryDeleteTag(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("DELETE", endpoint, nil)
	if strings.Compare(c.Authorization, "") != 0 {
		reqest.Header.Add("Authorization", c.Authorization)
	}

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode != 204 {
		e2e.Logf("delete %s failed, response code is %d", imageIndex, response.StatusCode)
		return false, nil
	}
	return true, nil
}

// DeleteTag will delete the image
func (c *QuayCLI) DeleteTag(imageIndex string) (bool, error) {
	rc, error := c.TryDeleteTag(imageIndex)
	if rc != true {
		e2e.Logf("try to delete %s again", imageIndex)
		rc, error = c.TryDeleteTag(imageIndex)
		if rc != true {
			e2e.Failf("delete tag failed on quay.io")
		}
	}
	return rc, error
}

func (c *QuayCLI) CheckTagNotExist(imageIndex string) (bool, error) {
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Replace(imageIndex, ":", "/tag/", 1)
	}
	endpoint := c.EndPointPre + imageIndex + "/images"
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	reqest.Header.Add("Authorization", c.Authorization)

	if err != nil {
		return false, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return false, err
	}
	if response.StatusCode == 404 {
		e2e.Logf("tag %s not exist", imageIndex)
		return true, nil
	} else {
		contents, _ := ioutil.ReadAll(response.Body)
		e2e.Logf("responce is %s", string(contents))
		return false, nil
	}
}

func (c *QuayCLI) GetTagNameList(imageIndex string) ([]string, error) {
	var TagNameList []string
	tags, err := c.GetTags(imageIndex)
	if err != nil {
		return TagNameList, err
	}
	for _, tagIndex := range tags {
		TagNameList = append(TagNameList, tagIndex.Name)
	}
	return TagNameList, nil
}

func (c *QuayCLI) GetTags(imageIndex string) ([]TagInfo, error) {
	var result []TagInfo
	if strings.Contains(imageIndex, ":") {
		imageIndex = strings.Split(imageIndex, ":")[0] + "/tag/"
	}
	if strings.Contains(imageIndex, "/tag/") {
		imageIndex = strings.Split(imageIndex, "tag/")[0] + "tag/"
	}
	endpoint := c.EndPointPre + imageIndex
	e2e.Logf("endpoint is %s", endpoint)

	client := &http.Client{}
	reqest, err := http.NewRequest("GET", endpoint, nil)
	reqest.Header.Add("Authorization", c.Authorization)
	if err != nil {
		return result, err
	}
	response, err := client.Do(reqest)
	defer response.Body.Close()
	if err != nil {
		return result, err
	}
	e2e.Logf("%s", response.Status)
	if response.StatusCode != 200 {
		e2e.Logf("get %s failed, response code is %d", imageIndex, response.StatusCode)
		return result, fmt.Errorf("return code is %d, not 200", response.StatusCode)
	} else {
		contents, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return result, err
		}
		//e2e.Logf(string(contents))
		//unmarshal json file
		var TagsResultOut TagsResult
		if err := json.Unmarshal(contents, &TagsResultOut); err != nil {
			return result, err
		}
		result = TagsResultOut.Tags
		return result, nil
	}
}
