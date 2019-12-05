package dev

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"github.com/hong008/wechat-sdk/pkg/util"
	"io"
	"reflect"
	"strings"
)

type refundNotifyResult struct {
	ReturnCode string `xml:"return_code"`
	ReturnMsg  string `xml:"return_msg"`
	Appid      string `xml:"appid"`
	MchId      string `xml:"mch_id"`
	NonceStr   string `xml:"nonce_str"`
	ReqInfo    string `xml:"req_info"`

	//加密字段
	TransactionId       string `xml:"transaction_id"`
	OutTradeNo          string `xml:"out_trade_no"`
	RefundId            string `xml:"refund_id"`
	OutRefundNo         string `xml:"out_refund_no"`
	TotalFee            int64  `xml:"total_fee"`
	SettlementRefundFee int64  `xml:"settlement_refund_fee"`
	RefundStatus        string `xml:"refund_status"`
	SuccessTime         string `xml:"success_time"`
	RefundRecvAccout    string `xml:"refund_recv_accout"`
	RefundAccount       string `xml:"refund_account"`
	RefundRequestSource string `xml:"refund_request_source"`
}

func (r *refundNotifyResult) Param(key string) (interface{}, error) {
	var err error
	switch key {
	case "return_code":
		return r.ReturnCode, err
	case "return_msg":
		return r.ReturnMsg, err
	case "appid":
		return r.Appid, err
	case "mch_id":
		return r.MchId, err
	case "nonce_str":
		return r.NonceStr, err
	case "req_info":
		return r.ReqInfo, err
	case "transaction_id":
		return r.TransactionId, err
	case "out_trade_no":
		return r.OutTradeNo, err
	case "total_fee":
		return r.TotalFee, err
	case "refund_id":
		return r.RefundId, err
	case "out_refund_no":
		return r.OutRefundNo, err
	case "settlement_refund_fee":
		return r.SettlementRefundFee, err
	case "refund_status":
		return r.RefundStatus, err
	case "success_time":
		return r.SuccessTime, err
	case "refund_recv_accout":
		return r.RefundRecvAccout, err
	case "refund_account":
		return r.RefundAccount, err
	case "refund_request_source":
		return r.RefundRequestSource, err
	default:
		err = errors.New(fmt.Sprintf("invalid key: %s", key))
		return nil, err
	}
}

func (r refundNotifyResult) ListParam() Params {
	p := make(Params)

	t := reflect.TypeOf(r)
	v := reflect.ValueOf(r)

	for i := 0; i < t.NumField(); i++ {
		if !v.Field(i).IsZero() {
			tagName := strings.Split(string(t.Field(i).Tag), "\"")[1]
			p[tagName] = v.Field(i).Interface()
		}
	}
	return p
}

func (m *myPayer) RefundNotify(body io.ReadCloser) (ResultParam, error) {
	if body == nil {
		return nil, errors.New("body is nil")
	}

	var result *refundNotifyResult
	err := xml.NewDecoder(body).Decode(&result)
	if err != nil {
		return nil, err
	}

	//对结果中对req_info执行解密：
	//1. 对加密串A做base64解码，得到加密串B
	//2. 对商户key做md5，得到32位小写key*
	//3. 用key*对加密串B做AES-256-ECB解密（PKCS7Padding）
	if result.ReqInfo == "" {
		return nil, errors.New("wx response without req_info")
	}
	reqInfo := base64.StdEncoding.EncodeToString([]byte(result.ReqInfo))
	md5Key := strings.ToLower(util.SignMd5(m.apiKey))
	realData, err := util.AES256ECBDecrypt([]byte(reqInfo), []byte(md5Key))
	if err != nil {
		return nil, err
	}
	err = xml.Unmarshal(realData, &result)
	if err != nil {
		fmt.Printf("\n1111 %v\n", err)
		return nil, err
	}
	return result, nil
}
