package wechat_sdk

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	uuid "github.com/satori/go.uuid"
	"github.com/wechat-sdk/pkg/e"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	//统一下单必须参数
	UnifiedOrderMustParam = []string{"appid", "mch_id", "nonce_str", "sign", "body", "out_trade_no", "total_fee", "spbill_create_ip", "notify_url", "trade_type"}
	//统一下单可选参数
	UnifiedOrderOptionalParam = []string{"device_info", "sign_type", "detail", "attach", "fee_type", "time_start", "time_expire", "goods_tag", "limit_pay", "receipt"}
)

type WeClient interface {
	SetParams(map[string]interface{})
	AddParam(key string, value interface{})
	AddParams(map[string]interface{})
	DelParam(key string)

	GetWxOpenId(code string) (Token, error)
	GetWxUserInfo(lang string) (User, error)
	ShareH5(string, ...string) (*H5Response, error)
	DoUnifiedOrder() (UnifiedOrder, error)
	DoQueryOrder()
}

type myWeClient struct {
	sync.Mutex                       //params读写锁
	params    map[string]interface{} //用于存放请求接口时需要传入的参数
	appId     string                 //appId
	appSecret string                 //app密钥
	mchId     string                 //商户号
}

var (
	c *myWeClient
)

func NewClientWithParam(appId, appSecret, mchId string) WeClient {
	c = &myWeClient{
		params:    make(map[string]interface{}),
		appId:     appId,
		appSecret: appSecret,
		mchId:     mchId,
	}
	return c
}

func (client *myWeClient) SetParams(param map[string]interface{}) {
	client.Lock()
	if param != nil {
		client.params = param
	}
	client.Unlock()
}

func (client *myWeClient) AddParam(key string, value interface{}) {
	client.Lock()
	if client.params == nil {
		client.params = make(map[string]interface{})
	}
	client.params[key] = value
	client.Unlock()
}

func (client *myWeClient) AddParams(params map[string]interface{}) {
	client.Lock()
	if client.params == nil {
		client.params = make(map[string]interface{})
	}
	if params != nil {
		for k, v := range params {
			client.params[k] = v
		}
	}
	client.Unlock()
}

func (client *myWeClient) DelParam(key string) {
	client.Lock()
	if client.params == nil {
		return
	}
	delete(client.params, key)
	client.Unlock()
}

func (client *myWeClient) checkBaseParam() error {
	//校验参数
	if client.appId == "" || client.appSecret == "" || client.mchId == "" {
		return e.ErrorInitClient
	}
	return nil
}

//获取微信openId
//code: 用户同意授权后，前端获取的code
//后端可以校验微信后台设置的state
func (client *myWeClient) GetWxOpenId(code string) (token Token, err error) {
	if err = client.checkBaseParam(); err != nil {
		return
	}
	//拉取网页授权token的api
	apiUrl := fmt.Sprintf("https://api.weixin.qq.com/sns/oauth2/access_token?appid=%v&secret=%v&code=%v&grant_type=authorization_code", client.appId, client.appSecret, code)

	response, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var data *tokenInfo
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	if data.errMsg != "" || data.errCode != 0 {
		return nil, errors.New(data.errMsg)
	}
	//获取到openId后放入client中
	params := map[string]interface{}{
		"openid":       data.openId,
		"access_token": data.authAccessToken,
	}

	client.AddParams(params)

	return data, nil
}

//获取微信用户信息
//lang: 返回国家地区版本, zh_CN 简体，zh_TW 繁体，en 英语
func (client *myWeClient) GetWxUserInfo(lang string) (User, error) {
	if client.params == nil {
		return nil, e.ErrorNilParam
	}

	openId, ok := client.params["openid"]
	if !ok {
		return nil, e.ErrorNoOpenId
	}
	accessToken, ok := client.params["access_token"]
	if !ok {
		return nil, e.ErrorNoToken
	}
	//获取用户基本信息的API url
	apiUrl := fmt.Sprintf("https://api.weixin.qq.com/sns/userinfo?access_token=%v&openid=%v&lang=%v", accessToken, openId, lang)

	response, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var info *userInfo
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, err
	}
	//校验是否正确返回了数据
	if info.errMsg != "" || info.errCode != 0 {
		return nil, errors.New(info.errMsg)
	}
	return info, nil
}

//微信H5页面分享
//url:待分享的页面url
func (client *myWeClient) ShareH5(url string, nonceStrs ...string) (*H5Response, error) {
	ticket, err := getTicketFromWx()
	if err != nil {
		return nil, err
	}

	//时间戳
	now := time.Now().Unix()

	nonceStr := ""
	if len(nonceStrs) <= 0 {
		//没有传入随机字符串的话，则生成随机字符串
		u4, _ := uuid.NewV4()
		s := strings.Replace(fmt.Sprintf("%s", u4), "-", "", -1)
		nonceStr = s[:16]
	} else {
		nonceStr = nonceStrs[0]
	}

	//签名
	t := sha1.New()
	toSignStr := fmt.Sprintf("jsapi_ticket=%v&noncestr=%v&timestamp=%vurl=%v", ticket.Ticket, nonceStr, now, url)
	io.WriteString(t, toSignStr)
	sign := fmt.Sprintf("%x", t.Sum(nil))

	resp := &H5Response{
		AppID:     client.appId,
		TimeStamp: now,
		NonceStr:  nonceStr,
		Signature: sign,
	}
	return resp, nil
}

//统一下单
func (client *myWeClient) DoUnifiedOrder() (UnifiedOrder, error) {
	//校验参数
	if err := client.checkBaseParam(); err != nil {
		return nil, err
	}
	if client.params == nil {
		return nil, e.ErrorNilParam
	}

	//校验是否缺少必需的参数，以及将统一下单的参数放入params中
	params := make(map[string]interface{})
	var paramNames []string
	for _, k := range UnifiedOrderMustParam {
		//sign不用传入
		if k == "sign" {
			continue
		}
		if _, ok := client.params[k]; !ok {
			return nil, errors.New(fmt.Sprintf("lack of param: %v", k))
		}

		params[k] = client.params[k]
		paramNames = append(paramNames, k)
	}

	for _, k := range UnifiedOrderOptionalParam {
		if _, ok := client.params[k]; ok {
			params[k] = client.params[k]
			paramNames = append(paramNames, k)
		}
	}

	//参数名排序
	sort.Strings(paramNames)
	//参数拼接
	toBeSignStr := ""

	for i, n := range paramNames {
		str := ""
		if i == 0 {
			str = fmt.Sprintf("%v=%v", n, params[n])
		} else {
			str = fmt.Sprintf("&%v=%v", n, params[n])
		}
		toBeSignStr += str
	}

	//在待签名的字符串后追加appSecret
	toBeSignStr += fmt.Sprintf("&key=%v", client.appSecret)
	signValue := ""
	//根据签名方式签名
	if signType, ok := params["sign_type"]; ok && signType.(string) == "HMAC-SHA256" {
		//HMAC-SHA256
		signValue = strings.ToUpper(signHMACSHA256(toBeSignStr))
	} else {
		//默认用MD5签名
		params["sign_type"] = "MD5"
		signValue = strings.ToUpper(signMd5(toBeSignStr))
	}
	params["sign"] = signValue

	writer := bytes.NewBuffer(make([]byte, 0))
	maps(params).marshalXML(writer)
	result, err := postUnifiedOrder(e.UnifiedOrderApiUrl, "application/xml;charset=utf-8", writer)
	if err != nil {
		return nil, err
	}
	if result.sign != signValue {
		return nil, errors.New("sign wrong")
	}

	return result, nil
}

//查询订单
func (client *myWeClient) DoQueryOrder() {

}