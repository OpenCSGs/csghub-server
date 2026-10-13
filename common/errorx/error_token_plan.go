package errorx

import (
	"database/sql"
	"errors"
)

const errTokenPlanPrefix = "TOKENPLAN-ERR"

const (
	tokenPlanError = iota
	tokenPlanInUsing
	tokenPlanPriceLower
	tokenPlanLatestPeriodNotPaid
)

var (
	// token plan error need retry or contact admin to check
	//
	// Description: Please retry later or contact admin to check TokenPlan error.
	//
	// en-US: Please retry later or contact admin to check Token plan error
	//
	// zh-CN: 请稍后重试或联系管理员处理TokenPlan错误
	//
	// zh-HK: 请稍后重试或联系管理員处理TokenPlan錯誤
	ErrTokenPlanError error = CustomError{prefix: errTokenPlanPrefix, code: tokenPlanError}

	// token plan is in using
	//
	// Description: The token plan in request is already in using.
	//
	// Description_ZH: TokenPlan已在使用
	//
	// en-US: Token plan is in using
	//
	// zh-CN: TokenPlan已在使用
	//
	// zh-HK: TokenPlan已在使用
	ErrTokenPlanInUsing error = CustomError{prefix: errTokenPlanPrefix, code: tokenPlanInUsing}

	// token plan price lower
	//
	// Description: The token plan price in request is lower than the current plan price.
	//
	// Description_ZH: TokenPlan价格低于当前周期价格
	//
	// en-US: Token plan price lower lower than the current plan price
	//
	// zh-CN: TokenPlan价格低于当前周期价格
	//
	// zh-HK: TokenPlan 價格低於當前週期價格
	ErrTokenPlanPriceLower error = CustomError{prefix: errTokenPlanPrefix, code: tokenPlanPriceLower}

	// token plan latest period is not paid
	//
	// Description: The token plan latest period in request is not paid.
	//
	// Description_ZH: TokenPlan最新周期未支付
	//
	// en-US: Token plan latest period is not paid
	//
	// zh-CN: TokenPlan最新周期未支付
	//
	// zh-HK: TokenPlan 最新週期未支付
	ErrTokenPlanLatestPeriodNotPaid error = CustomError{prefix: errTokenPlanPrefix, code: tokenPlanLatestPeriodNotPaid}
)

func HandleTokenPlanError(err error, ctx map[string]interface{}) error {
	if err == nil {
		return nil
	}
	customErr := CustomError{
		prefix:  errTokenPlanPrefix,
		context: ctx,
		err:     err,
	}
	if errors.Is(err, sql.ErrNoRows) {
		customErr.prefix = errSysPrefix
		customErr.code = int(databaseNoRows)
		return customErr
	} else {
		customErr.code = int(tokenPlanError)
		return customErr
	}
}
