package rest

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/SmartMeshFoundation/matrix-regservice/utils"

	"github.com/ethereum/go-ethereum/common"

	"github.com/SmartMeshFoundation/matrix-regservice/models"

	"github.com/SmartMeshFoundation/matrix-regservice/params"

	"github.com/SmartMeshFoundation/SmartRaiden/log"
	sutils "github.com/SmartMeshFoundation/SmartRaiden/utils"
	"github.com/ant0ine/go-json-rest/rest"
)

/*
{
    "localpart": "someone3",
    "displayname": "someone interesting",
    "duration_seconds": 200,
    "password_hash":"aaaaaaaaaaa"
}
*/
type reg struct {
	LocalPart    string `json:"localpart"`     //@someone:matrix.org someone is localpoart,matrix.org is domain
	DisplayName  string `json:"displayname"`   // displayname of this user
	PasswordHash string `json:"password_hash"` // password hash calc using bcrypt
	Password     string `json:"password,omitempty"`
}

/*
{
    "access_token": "MDAyZGxvY2F0aW9uIHRyYW5zcG9ydDAxLnNtYXJ0cmFpZGVuLm5ldHdvcmsKMDAxM2lkZW50aWZpZXIga2V5CjAwMTBjaWQgZ2VuID0gMQowMDNjY2lkIHVzZXJfaWQgPSBAc29tZW9uZTM6dHJhbnNwb3J0MDEuc21hcnRyYWlkZW4ubmV0d29yawowMDE2Y2lkIHR5cGUgPSBhY2Nlc3MKMDAyMWNpZCBub25jZSA9IDF0Wml3MlFVcnlaYUtiaGoKMDAyZnNpZ25hdHVyZSD4fe93M_-P1qUD0nnFKUV7JyI6Jv02kLXaDZLu-gBUFwo",
    "home_server": "transport01.smartraiden.network",
    "user_id": "@someone3:transport01.smartraiden.network"
}
*/
type regResp struct {
	AccessToken string `json:"access_token"`
	HomeServer  string `json:"home_server"`
	UserID      string `json:"user_id"`
}

//RegisterUser new user for homeserver
func RegisterUser(w rest.ResponseWriter, r *rest.Request) {
	var reg reg
	err := r.DecodeJsonPayload(&reg)
	if err != nil {
		log.Error(err.Error())
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(reg.LocalPart) != len(common.Address{}.String()) {
		rest.Error(w, fmt.Sprintf("localpart length err got=%s", reg.LocalPart), http.StatusBadRequest)
		return
	}
	verifyReg(&reg, w)
}
func verifyReg(r *reg, w rest.ResponseWriter) {
	log.Trace(fmt.Sprintf("reg=%s", sutils.StringInterface(r, 3)))
	err := verifyPassword(r)
	if err != nil {
		log.Error(err.Error())
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = verifyDisplayName(r)
	if err != nil {
		log.Error(err.Error())
		rest.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if models.IsUserAlreadyExists(r.LocalPart) {
		rest.Error(w, fmt.Sprintf("already exists"), http.StatusConflict)
		return
	}
	resp, err := registerOnHomeServer(r)
	if err != nil {
		log.Error(err.Error())
		rest.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	err = w.WriteJson(resp)
	if err != nil {
		log.Error(err.Error())
	}
}
func verifyPassword(r *reg) error {
	signature, err := hex.DecodeString(r.Password)
	if err != nil {
		return err
	}
	addr := common.HexToAddress(r.LocalPart)
	return utils.VerifyPasswordSignature(addr, signature)
}
func getSignatureFromDisplayName(displayName string) (signature []byte, err error) {
	ss := strings.Split(displayName, "-")
	//userAddr-Signature
	if len(ss) != 2 {
		err = fmt.Errorf("display name format error %s", displayName)
		return
	}
	//signature length is 130
	if len(ss[1]) != 130 {
		err = fmt.Errorf("signature error")
	}
	signature, err = hex.DecodeString(ss[1])
	return
}
func verifyDisplayName(r *reg) error {
	signature, err := getSignatureFromDisplayName(r.DisplayName)
	if err != nil {
		return err
	}
	return utils.VerifyDisplayName(r.LocalPart, signature)
}

var hclient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost:   100,
		ResponseHeaderTimeout: time.Second * 30,
	},
}

func registerOnHomeServer(r *reg) (resp *regResp, err error) {
	var req *http.Request
	r.PasswordHash = utils.HashPasswordWrapper(r.Password)
	r.Password = ""
	jsonStr, err := json.Marshal(r)
	if err != nil {
		return
	}
	url := fmt.Sprintf("%s?access_token=%s", params.MatrixRegisterUrl, params.ASToken)
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonStr))
	log.Trace(fmt.Sprintf("registeronhomeserver url=%s,body=%s",
		url, string(jsonStr)))
	res, err := hclient.Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	resp = &regResp{}
	err = json.Unmarshal(body, resp)
	if err != nil {
		return
	}
	return
}
