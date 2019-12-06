package dev

import (
	"errors"
	"fmt"

	"github.com/hong008/wechat-sdk/pkg/e"
	"github.com/hong008/wechat-sdk/pkg/util"
)

/*
	查询退款
*/

var (
	refundQueryMustParam     = []string{"appid", "mch_id", "nonce_str", "sign"}
	refundQueryOneParam      = []string{"transaction_id", "out_trade_no", "out_refund_no", "refund_id"}
	refundQueryOptionalParam = []string{"sign_type", "offset"}
)

const queryRefundUrl = "https://api.mch.weixin.qq.com/pay/refundquery"

func (m *myPayer) RefundQuery(param Param) (ResultParam, error) {
	if param == nil {
		return nil, errors.New("param is empty")
	}
	if err := m.checkForPay(); err != nil {
		return nil, err
	}
	param.Add("appid", m.appId)
	param.Add("mch_id", m.mchId)

	var signType = e.SignTypeMD5
	var count = 0
	for _, k := range refundQueryOneParam {
		if v := param.Get(k); v != nil {
			count++
		}
	}
	if count == 0 {
		return nil, errors.New("need one of refund_id/out_refund_no/transaction_id/out_trade_no")
	} else if count > 1 {
		return nil, errors.New("more than one with refund_id/out_refund_no/transaction_id/out_trade_no")
	}

	for _, k := range refundQueryMustParam {
		if k == "sign" {
			continue
		}
		if param.Get(k) == nil {
			return nil, errors.New(fmt.Sprintf("need %s", k))
		}
	}

	for k := range param {
		if k == "sign_type" {
			signType = param[k].(string)
		}
		if !util.HaveInArray(refundQueryMustParam, k) && !util.HaveInArray(refundQueryOptionalParam, k) && !util.HaveInArray(refundQueryOneParam, k) {
			return nil, errors.New(fmt.Sprintf("no need %s", k))
		}
	}

	sign := param.Sign(signType)
	param.Add("sign", sign)

	reader, err := param.MarshalXML()
	if err != nil {
		return nil, err
	}

	var request = &postRequest{
		Body:        reader,
		Url:         queryRefundUrl,
		ContentType: "application/xml;charset=utf-8",
	}

	result, err := postToWx(request)
	if err != nil {
		return nil, err
	}

	if returnCode, _ := result.GetString("return_code"); returnCode != "SUCCESS" {
		returnMsg, _ := result.GetString("return_msg")
		return nil, errors.New(returnMsg)
	}
	if resultCode, _ := result.GetString("result_code"); resultCode != "SUCCESS" {
		errDes, _ := result.GetString("err_code_des")
		return nil, errors.New(errDes)
	}
	sign = result.Sign(signType)
	if resultSign, _ := result.GetString("sign"); resultSign != sign {
		return nil, e.ErrCheckSign
	}
	return result, nil
}
