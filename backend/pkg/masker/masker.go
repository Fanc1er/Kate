// Package masker 提供敏感数据三时机脱敏（入库前/API 返回前/报告生成时）。
package masker

import "regexp"

var (
	idCardRe   = regexp.MustCompile(`\b\d{15}|\d{17}[\dXx]\b`)
	phoneRe    = regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	emailRe    = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	accessKeyRe = regexp.MustCompile(`(AKIA[0-9A-Z]{16}|LTAI[0-9A-Za-z]{12,20}|sk-[0-9A-Za-z]{16,48})`)
)

// MaskString 对单段文本执行身份证/手机号/邮箱/AccessKey 脱敏。
func MaskString(s string) string {
	if s == "" {
		return s
	}
	s = idCardRe.ReplaceAllStringFunc(s, maskIDCard)
	s = phoneRe.ReplaceAllStringFunc(s, maskPhone)
	s = emailRe.ReplaceAllStringFunc(s, maskEmail)
	s = accessKeyRe.ReplaceAllStringFunc(s, maskAccessKey)
	return s
}

// MaskID 脱敏身份证号，保留前 4 后 4。
func maskIDCard(id string) string {
	if len(id) < 8 {
		return "***"
	}
	return id[:4] + "**********" + id[len(id)-4:]
}

// MaskPhone 脱敏手机号，保留前 3 后 4。
func maskPhone(p string) string {
	if len(p) < 7 {
		return "***"
	}
	return p[:3] + "****" + p[len(p)-4:]
}

// MaskEmail 脱敏邮箱，保留首字符与域名。
func maskEmail(e string) string {
	at := regexp.MustCompile(`@`).FindStringIndex(e)
	if at == nil {
		return "***"
	}
	local := e[:at[0]]
	domain := e[at[1]:]
	if len(local) <= 1 {
		local = "*"
	} else {
		local = local[:1] + "***"
	}
	return local + "@" + domain
}

func maskAccessKey(k string) string {
	if len(k) <= 8 {
		return "***"
	}
	return k[:4] + "****" + k[len(k)-4:]
}
