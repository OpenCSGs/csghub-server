package rsa

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mockrsa "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rsa"
	"opencsg.com/csghub-server/common/types"
)

var (
	TestPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAntSWxxZr+qhshYAdx1IIgpAbXXIzVF8FMIoGU5NEVM4ws7CB
4uEj61Mnf9AwwHzsrshmTAFFWy+3IrIPHhcFgs4cS+UFaZSy9VoeG6jsAPmE3Za4
NsFU6WOGKAvB0KixnitEIt5aCtTg4J2OxuSt8V3McgfI3I4Ef7p4CbLmMg6Dqycd
CHQ8e09iez3P4q28dgJdTXuHYTb89F0ykOzjKclUoIk0m8IS3yfLuRaOw6cUaT1r
OXKytfzDWsqjm/VsipQGOKQppC8IH/GUBhGc1E+5lOwW16HjEO5hreY2+baLFqm+
dM8J3/1G3LCfU4sTmMoiV3+eq+toIOk2itBFrwIDAQABAoIBAQCITcR/YhyUZcmL
3+CuVxX6hhUV4pVuSIU5nJnFS1KOvMxKyKUOwUuD/j6dj1rnNc4gSNaRT13n3VD6
s3gJyJPwJ1VdRFOawgO6TvYboqG2TGvbhibcxplKGSDeaQiROtQC+vpkOhFnzjyW
RmBrC4DC7E4xZcDYlgACZVHycNbgCP6PykvcdvyP2rDJlUCWYNhwfC3p+wo5HLdz
rSJZlZtQvGfhT9QsnewaSoId8WQ6qUZ2fLpOEYAd2oouZxATeK5Urteo/jSSr+wm
tdUdVZU+NZobEoQjZN41dAaQ0ooNUDWmVpX5qc4K1eY41wryF+jln8Qt0/lWqCJJ
iM4sFxnRAoGBAMyZ9hmFhlyCZwGMlTyi6hO5dTHlXeI5v0lYOBOvLXcNYqvegR7E
pTkcm6g7fO3eCKyCnk0XL58dXrKWxUEpJepUD12hoHrmPZ/dQIOkwGhtfbTbq1+9
Xc3hmN8z2AiOWLO+1wN0uBZrf+ICzMFJpc33M0y+pA/KXfZNKTbsA+L1AoGBAMa7
EXsQgNQK+PZbLBfJ8vwOEhQ/IvOZNjVVUSUmoAnT64pafVBKvGxDCnBgeu4/0ZSp
9+/nDbqmDR5/peCcl5+AeptRVjCSf66sX9yFbXyX7hVJ+kjw/Gvy9Vu1y/er6Mil
wd6MZzV9MWuX7espJzafIygoo01cNOPzSei2N0eTAoGADKc03g0w6wWxgxoDnLVi
joe4pLZPoQ77Mnj/NtBtmmA8iu5+w71bjnWjdrr/FeLWXHzTd2cIreluEtNaLZZy
3tQGAz9col0c0IcpVzrYH10uGgI/zfLzGylpf9w/7v+Gos8ZkwAj5lcNmJedvBJm
657vED71/HgCaZoKA3iDIQUCgYBC1sQIWgKaTp5xbTSlQ5zfvXPDL4D34T3kLi++
iQEmjQoZXFntWVWKK/ok1a5C12AL2iazn0h00Eh1S4VkyAYO9U1HU9HjQEKFYyBS
sOWkFA1VR65QPg0H2Y1ALSLOyBjg8y8DRMGpsdOfVlgE0bCIpHlUlmZmLG71g+wF
wtNQ1QKBgDQkJTsJdsOi7MOHNgBMZD3N8VjqvKYeXRDJVwn7P+0IsUYohUF6l3Nm
NnrZ7uPrWXPebs2vC+JQz7hheM9oFN0oZjqqzBKcU6LRO1zgbsG7dgAkPm0Nn975
yQQzm3HN4l6LYY7QvEEhL7UR6UTg973HX0XSm0ju2Il8rQmNtxgy
-----END RSA PRIVATE KEY-----`

	TestPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAntSWxxZr+qhshYAdx1II
gpAbXXIzVF8FMIoGU5NEVM4ws7CB4uEj61Mnf9AwwHzsrshmTAFFWy+3IrIPHhcF
gs4cS+UFaZSy9VoeG6jsAPmE3Za4NsFU6WOGKAvB0KixnitEIt5aCtTg4J2OxuSt
8V3McgfI3I4Ef7p4CbLmMg6DqycdCHQ8e09iez3P4q28dgJdTXuHYTb89F0ykOzj
KclUoIk0m8IS3yfLuRaOw6cUaT1rOXKytfzDWsqjm/VsipQGOKQppC8IH/GUBhGc
1E+5lOwW16HjEO5hreY2+baLFqm+dM8J3/1G3LCfU4sTmMoiV3+eq+toIOk2itBF
rwIDAQAB
-----END PUBLIC KEY-----`

	EncodedResult = `-----BEGIN LICENSE KEY-----
L/+FAwEBB1JTQUluZm8B/4YAAQIBB1BheWxvYWQBCgABCVNpZ25hdHVyZQEKAAAA
/gJo/4YB/gFdLX8DAQEKUlNBUGF5bG9hZAH/gAABAgEDS2V5AQwAAQhEYXRhQm9k
eQH/ggAAAP+F/4EDAQEIRGF0YUJvZHkB/4IAAQkBB0NvbXBhbnkBDAABBUVtYWls
AQwAAQdQcm9kdWN0AQwAAQdFZGl0aW9uAQwAAQdNYXhVc2VyAQQAAQlTdGFydFRp
bWUB/4QAAQpFeHBpcmVUaW1lAf+EAAEFRXh0cmEBDAABB1ZlcnNpb24BDAAAABD/
gwUBAQRUaW1lAf+EAAAA/5X/gAEkZTUxOWZhNWEtNzZmZC00NTA2LWFlOTktODc2
ZjQxZjZiZTQ5AQEHb3BlbmNzZwESd2FuZ2hoMjAwM0AxNjMuY29tAQZDU0dIdWIB
CkVudGVycHJpc2UBZAEPAQAAAA7evWPEAAAAAP//AQ8BAAAADt7k8MQAAAAA//8B
E3sidG9rZW5fbGltaXQiOiAxMH0AAAH+AQCLedVGlnShSBdGWwsX5c8dcKikTB1w
XTScIiOZImcmWihDEH/WfvK0gAUVtCz0Hebuux5MVL4mQwCyBkAYN4E3nz2f3qpV
YxQVI5IXxDOBqO15pmcOfXIeUppYNG4zIoiSLA7T47BYMW1R1Z+SyllSQ+Dy2qjx
JFmoVy0oCzfEZGIi5iOGulppIKAN6EXMaZq9KZ9ns4+0Njjiep6Uuhx0GUm3UIaN
ujqKwWJ0SWImnZmqcuETIw17v3nykrs3j7YwK1HP3vwi1Mrr1xiY7g6QDuj4GrFK
mvbMe42afoH/Wj+s/lLA4T6nAfOrzuxcO6vXBZrLBhKYHp2uqyAqM3wHAA==
-----END LICENSE KEY-----`
	TestLicenseCompany = "opencsg"
	TestLicenseEmail   = "wanghh2003@163.com"
	TestLicenseProduct = "CSGHub"
	TestLicenseEdition = "Enterprise"
	TestLicenseMaxUser = 50
	TestLicenseExtra   = "{\"token_limit\": 10}"
	TestLicenseKey     = "e519fa5a-76fd-4506-ae99-876f41f6be49"
)

func TestRSA_GenerateData(t *testing.T) {
	layout := "2006-01-02 15:04:05"
	startTime, err := time.Parse(layout, "2024-11-06 13:19:00")
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	expiredTime, err := time.Parse(layout, "2024-12-06 13:19:00")
	if err != nil {
		t.Fatalf("failed to parse time: %v", err)
	}
	payload := types.RSAPayload{
		Key: TestLicenseKey,
		DataBody: types.DataBody{
			Company:    TestLicenseCompany,
			Email:      TestLicenseEmail,
			Product:    TestLicenseProduct,
			Edition:    TestLicenseEdition,
			MaxUser:    TestLicenseMaxUser,
			StartTime:  startTime,
			ExpireTime: expiredTime,
			Extra:      TestLicenseExtra,
		},
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomInt := r.Intn(10000)

	filePath := fmt.Sprintf("/tmp/private_key_%d.pem", randomInt)

	mockReader := mockrsa.NewMockKeysReader(t)
	mockReader.EXPECT().ReadKey(filePath).Return([]byte(TestPrivateKey), nil)

	encoded, err := GenerateData(mockReader, filePath, payload)
	if err != nil {
		t.Errorf("failed to generate encoded data with error %v", err)
	}
	assert.Equal(t, EncodedResult, encoded)
}

func TestRSA_VerifyData(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomInt := r.Intn(10000)

	filePath := fmt.Sprintf("/tmp/public_key_%d.pem", randomInt)

	mockReader := mockrsa.NewMockKeysReader(t)
	mockReader.EXPECT().ReadKey(filePath).Return([]byte(TestPublicKey), nil)

	decoded, err := VerifyData(mockReader, filePath, EncodedResult)
	if err != nil {
		t.Errorf("fail to verify data with error %v", err)
	}

	assert.Equal(t, TestLicenseCompany, decoded.Company)
	assert.Equal(t, TestLicenseEmail, decoded.Email)
	assert.Equal(t, TestLicenseProduct, decoded.Product)
	assert.Equal(t, TestLicenseEdition, decoded.Edition)
	assert.Equal(t, TestLicenseMaxUser, decoded.MaxUser)
	assert.Equal(t, TestLicenseExtra, decoded.Extra)
}
