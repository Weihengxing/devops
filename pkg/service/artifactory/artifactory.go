package artifactory

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/pkg/errors"
	apiResource "github.com/yametech/devops/pkg/api/resource/artifactory"
	"github.com/yametech/devops/pkg/common"
	"github.com/yametech/devops/pkg/core"
	arResource "github.com/yametech/devops/pkg/resource/artifactory"
	"github.com/yametech/devops/pkg/service"
	"github.com/yametech/devops/pkg/utils"
	"github.com/yametech/go-flowrun"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"net/http"
	urlPkg "net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type ArtifactService struct {
	service.IService
}

func NewArtifact(i service.IService) *ArtifactService {
	return &ArtifactService{i}
}

func (a *ArtifactService) Watch(version string) (chan core.IObject, chan struct{}) {
	objectChan := make(chan core.IObject, 32)
	closed := make(chan struct{})
	a.IService.Watch(common.DefaultNamespace, common.Artifactory, string(arResource.ArtifactKind), version, objectChan, closed)
	return objectChan, closed
}

func (a *ArtifactService) List(name string, page, pageSize int64) ([]interface{}, int64, error) {
	offset := (page - 1) * pageSize
	filter := map[string]interface{}{}
	if name != "" {
		filter["spec.app_name"] = bson.M{"$regex": primitive.Regex{Pattern: ".*" + name + ".*", Options: "i"}}
	}
	sort := map[string]interface{}{
		"metadata.version": -1,
	}

	data, err := a.IService.ListByFilter(common.DefaultNamespace, common.Artifactory, filter, sort, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	count, err := a.IService.Count(common.DefaultNamespace, common.Artifactory, filter)
	if err != nil {
		return nil, 0, err
	}
	return data, count, nil

}

func (a *ArtifactService) Create(reqAr *apiResource.RequestArtifact) (*arResource.Artifact, error) {
	if IsChinese(reqAr.Branch) || IsChinese(reqAr.Tag) {
		return nil, errors.New("分支和tag不能为中文")
	}

	gitPath := ""
	if strings.Contains(reqAr.GitUrl, "http://") {
		sliceTemp := strings.Split(reqAr.GitUrl, "http://")
		gitPath = sliceTemp[len(sliceTemp)-1]
	} else if strings.Contains(reqAr.GitUrl, "https://") {
		sliceTemp := strings.Split(gitPath, "https://")
		gitPath = sliceTemp[len(sliceTemp)-1]
	}

	gitName := ""
	gitDirectory := ""
	if strings.Contains(gitPath, "/") {
		if sliceTemp := strings.Split(gitPath, "/"); len(sliceTemp) > 2 {
			gitDirectory = sliceTemp[len(sliceTemp)-2]
			gitName = sliceTemp[len(sliceTemp)-1]
		}
	}

	registry := ""
	if strings.Contains(reqAr.Registry, "http://") {
		sliceTemp := strings.Split(reqAr.Registry, "http://")
		registry = sliceTemp[len(sliceTemp)-1]
	} else if strings.Contains(reqAr.Registry, "https://") {
		sliceTemp := strings.Split(gitPath, "https://")
		registry = sliceTemp[len(sliceTemp)-1]
	} else {
		registry = reqAr.Registry
	}
	registry = fmt.Sprintf("%s/%s", registry, gitDirectory)
	imageUrl := fmt.Sprintf("%s/%s", registry, gitName)

	if len(reqAr.Tag) == 0 {
		reqAr.Tag = strings.ToLower(utils.NewSUID().String())
	}

	appName := fmt.Sprintf("%s-%d", reqAr.AppName, time.Now().UnixNano())

	ar := &arResource.Artifact{
		Metadata: core.Metadata{
			Name: appName,
		},
		Spec: arResource.ArtifactSpec{
			GitUrl:      reqAr.GitUrl,
			AppName:     reqAr.AppName,
			Branch:      reqAr.Branch,
			Tag:         reqAr.Tag,
			Remarks:     reqAr.Remarks,
			Language:    reqAr.Language,
			Registry:    registry,
			ProjectFile: reqAr.ProjectFile,
			ProjectPath: reqAr.ProjectPath,
			Images:      imageUrl,
		},
	}
	VerificationResults, err := CheckExists(ar)
	if !VerificationResults {
		return nil, err
	}
	ar.GenerateVersion()
	_, err = a.IService.Create(common.DefaultNamespace, common.Artifactory, ar)
	if err != nil {
		return nil, err
	}
	go a.SendCI(ar)
	return ar, nil
}

func CheckExists(ar *arResource.Artifact) (bool, error) {
	var HarborAddress string
	var catalogue string
	if strings.Contains(ar.Spec.Images, "/") {
		SliceTemp := strings.Split(ar.Spec.Registry, "/")
		HarborAddress = SliceTemp[0]
		catalogue = SliceTemp[1]
	}
	url := fmt.Sprintf("https://%s/api/v2.0/projects?project_name=%s", HarborAddress, catalogue)
	data := urlPkg.Values{}
	req, err := http.NewRequest("HEAD", url, strings.NewReader(data.Encode()))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(common.RegistryUser, common.RegistryPW)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		res, err := CreateProject(HarborAddress, catalogue)
		if err != nil {
			return res, err
		}

	}
	return true, nil
}

func CreateProject(HarborAddress, projectName string) (bool, error) {
	url := fmt.Sprintf("https://%s/api/v2.0/projects", HarborAddress)
	body := map[string]interface{}{
		"project_name": projectName,
		"metadata": map[string]interface{}{
			"public": "true",
		},
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(common.RegistryUser, common.RegistryPW)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 30 * time.Second, Transport: tr}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode == 201 {
		return true, nil
	}
	return false, errors.New("构建镜像仓库目录失败！")
}

func (a *ArtifactService) SendCI(ar *arResource.Artifact) {
	arCIInfo := &arResource.ArtifactCIInfo{
		Branch:      ar.Spec.Branch,
		CodeType:    ar.Spec.Language,
		CommitID:    ar.Spec.Tag,
		GitUrl:      ar.Spec.GitUrl,
		OutPut:      ar.Spec.Registry,
		ProjectPath: ar.Spec.ProjectPath,
		ProjectFile: ar.Spec.ProjectFile,
		RetryCount:  15,
		ServiceName: ar.Spec.AppName,
	}
	sendCIInfo, err := core.ToMap(arCIInfo)
	if err != nil {
		ar.Spec.ArtifactStatus = arResource.InitializeFail
		_, _, err = a.IService.Apply(common.DefaultNamespace, common.Artifactory, ar.UUID, ar, false)
		if err != nil {
			fmt.Printf("sendci initialize save error %s", err)
		}
		return
	}
	if !SendEchoer(ar.Metadata.UUID, common.EchoerCI, sendCIInfo) {
		ar.Spec.ArtifactStatus = arResource.InitializeFail
		_, _, err = a.IService.Apply(common.DefaultNamespace, common.Artifactory, ar.UUID, ar, false)
		if err != nil {
			fmt.Printf("sendci sendEchoer fail save error %s", err)
		}
		return
	}
	ar.Spec.ArtifactStatus = arResource.Building
	_, _, err = a.IService.Apply(common.DefaultNamespace, common.Artifactory, ar.UUID, ar, false)
	if err != nil {
		fmt.Printf("sendci sendEchoer success save error %s", err)
	}
}

func (a *ArtifactService) GetByUUID(uuid string) (*arResource.Artifact, error) {
	ar := &arResource.Artifact{}
	err := a.IService.GetByUUID(common.DefaultNamespace, common.Artifactory, uuid, ar)
	if err != nil {
		return nil, err
	}
	return ar, nil
}

func (a *ArtifactService) Update(uuid string, reqAr *apiResource.RequestArtifact) (core.IObject, bool, error) {
	ar := &arResource.Artifact{
		Spec: arResource.ArtifactSpec{
			GitUrl:   reqAr.GitUrl,
			AppName:  reqAr.AppName,
			Branch:   reqAr.Branch,
			Tag:      reqAr.Tag,
			Remarks:  reqAr.Remarks,
			Language: reqAr.Language,
			Registry: reqAr.Registry,
		},
	}
	ar.GenerateVersion()
	return a.IService.Apply(common.DefaultNamespace, common.Artifactory, uuid, ar, false)
}

func (a *ArtifactService) Delete(uuid string) error {
	err := a.IService.Delete(common.DefaultNamespace, common.Artifactory, uuid)
	if err != nil {
		return err
	}
	return nil
}

func SendEchoer(stepName string, actionName string, a map[string]interface{}) bool {
	if stepName == "" {
		fmt.Println("UUID is not none")
		return false
	}

	flowRun := &flowrun.FlowRun{
		EchoerUrl: common.EchoerUrl,
		Name:      fmt.Sprintf("%s_%d", common.DefaultNamespace, time.Now().UnixNano()),
	}
	flowRunStep := map[string]string{
		"SUCCESS": "done", "FAIL": "done",
	}

	flowRunStepName := fmt.Sprintf("%s_%s", actionName, stepName)
	flowRun.AddStep(flowRunStepName, flowRunStep, actionName, a)

	flowRunData := flowRun.Generate()
	fmt.Println(flowRunData)
	if !flowRun.Create(flowRunData) {
		fmt.Println("send fsm error")
		return false
	}
	return true
}

func (a *ArtifactService) GetBranch(gitpath string) ([]string, error) {
	url := fmt.Sprintf("http://%s:%s@%s", common.GitUser, common.GitPW, gitpath)
	r, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:          url,
		SingleBranch: false,
		NoCheckout:   true,
		Depth:        1,
	})
	if err != nil {
		return nil, err
	}

	sliceBranch := make([]string, 0)
	referenceIter, _ := r.References()
	err = referenceIter.ForEach(func(c *plumbing.Reference) error {
		if strings.Contains(string(c.Name()), "refs/remotes/origin/") {
			sliceTemp := strings.Split(string(c.Name()), "refs/remotes/origin/")
			sliceBranch = append(sliceBranch, sliceTemp[len(sliceTemp)-1])
		}
		return nil
	})
	return sliceBranch, err
}

func (a *ArtifactService) GetAppNumber(appName string) int {
	data, _, err := a.List(appName, 1, 0)
	if err != nil {
		return 0
	}
	b, err := json.Marshal(data)
	c := make([]*arResource.Artifact, 0)
	err = json.Unmarshal(b, &c)
	if err != nil {
		fmt.Println(err)
		return 0
	}
	var number = 0
	for _, v := range c {
		sliceName := strings.Split(v.Metadata.Name, "-")
		i, err := strconv.Atoi(sliceName[len(sliceName)-1])
		if err != nil {
			return 0
		}
		if i > number {
			number = i
		}
	}
	return number
}

func IsChinese(str string) bool {
	var count int
	for _, v := range str {
		if unicode.Is(unicode.Han, v) {
			count++
			break
		}
	}
	return count > 0
}
