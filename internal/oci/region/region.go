// Package region ports the Java oci-common enum RegionEnum: a static map
// between human-friendly region names (Chinese, e.g. "东京") and OCI region
// codes (e.g. "ap-tokyo-1"). tenant.region stores the friendly name; at OCI
// provider construction time it is converted to the code via CodeByName
// (parity with RegionEnum.getRegionCode used by OciUtils.getProvider).
package region

import "unicode"

// entry mirrors one RegionEnum constant. longName = RegionEnum.name,
// simpleName = RegionEnum.simpleName, code = RegionEnum.code.
type entry struct {
	longName   string
	simpleName string
	code       string
}

var entries = []entry{
	{"中东非洲-南非中部约翰内斯堡", "约翰内斯堡", "af-johannesburg-1"},
	{"中东非洲-摩洛哥卡萨布兰卡", "卡萨布兰卡", "af-casablanca-1"},
	{"亚太-韩国北部春川", "春川", "ap-chuncheon-1"},
	{"亚太-印度南部海得拉巴", "海得拉巴", "ap-hyderabad-1"},
	{"亚太-澳大利亚东南部墨尔本", "墨尔本", "ap-melbourne-1"},
	{"亚太-印度西部孟买", "孟买", "ap-mumbai-1"},
	{"亚太-日本中部大阪", "大阪", "ap-osaka-1"},
	{"亚太-韩国中部首尔", "首尔", "ap-seoul-1"},
	{"亚太-马来西亚古来", "古来", "ap-kulai-2"},
	{"亚太-新加坡", "新加坡", "ap-singapore-1"},
	{"亚太-印度尼西亚巴淡", "巴淡", "ap-batam-1"},
	{"亚太-新加坡西", "新加坡西", "ap-singapore-2"},
	{"亚太-澳大利亚东部悉尼", "悉尼", "ap-sydney-1"},
	{"亚太-日本东部东京", "东京", "ap-tokyo-1"},
	{"北美-加拿大东南部蒙特利尔", "蒙特利尔", "ca-montreal-1"},
	{"北美-加拿大东南部多伦多", "多伦多", "ca-toronto-1"},
	{"欧洲-荷兰西北部阿姆斯特丹", "阿姆斯特丹", "eu-amsterdam-1"},
	{"欧洲-德国中部法兰克福", "法兰克福", "eu-frankfurt-1"},
	{"欧洲-塞尔维亚中部乔万诺瓦茨", "乔万诺瓦茨", "eu-jovanovac-1"},
	{"欧洲-西班牙中部马德里-1", "马德里-1", "eu-madrid-1"},
	{"欧洲-西班牙中部马德里-3", "马德里-3", "eu-madrid-3"},
	{"欧洲-法国南部马赛", "马赛", "eu-marseille-1"},
	{"欧洲-意大利西北部米兰", "米兰", "eu-milan-1"},
	{"欧洲-意大利西北部都灵", "都灵", "eu-turin-1"},
	{"欧洲-法国中部巴黎", "巴黎", "eu-paris-1"},
	{"欧洲-瑞典中部斯德哥尔摩", "斯德哥尔摩", "eu-stockholm-1"},
	{"欧洲-瑞士北部苏黎世", "苏黎世", "eu-zurich-1"},
	{"欧洲-以色列中部耶路撒冷", "耶路撒冷", "il-jerusalem-1"},
	{"中东-阿联酋阿布扎比", "阿布扎比", "me-abudhabi-1"},
	{"中东-阿联酋迪拜", "迪拜", "me-dubai-1"},
	{"中东-沙特阿拉伯西部吉达", "吉达", "me-jeddah-1"},
	{"中东-沙特阿拉伯首都利雅得", "利雅得", "me-riyadh-1"},
	{"北美-墨西哥东北部蒙特雷", "蒙特雷", "mx-monterrey-1"},
	{"北美-墨西哥中部克雷塔罗", "克雷塔罗", "mx-queretaro-1"},
	{"南美-哥伦比亚中部波哥大", "波哥大", "sa-bogota-1"},
	{"南美-智利中部圣地亚哥", "圣地亚哥", "sa-santiago-1"},
	{"南美-巴西东部圣保罗", "圣保罗", "sa-saopaulo-1"},
	{"南美-巴西南部维涅杜", "维涅杜", "sa-vinhedo-1"},
	{"欧洲-英国西部加的夫", "加的夫", "uk-cardiff-1"},
	{"欧洲-英国南部伦敦", "伦敦", "uk-london-1"},
	{"北美-美国东部阿什本", "阿什本", "us-ashburn-1"},
	{"北美-美国中西部芝加哥", "芝加哥", "us-chicago-1"},
	{"北美-美国西部凤凰城", "凤凰城", "us-phoenix-1"},
	{"北美-美国西部圣何塞", "圣何塞", "us-sanjose-1"},
	{"南美-智利西部瓦尔帕莱索", "瓦尔帕莱索", "sa-valparaiso-1"},
}

var (
	byLong   = make(map[string]string, len(entries)) // long name → code
	bySimple = make(map[string]string, len(entries)) // simple name → code
	byCode   = make(map[string]string, len(entries)) // code → simple name
)

func init() {
	for _, e := range entries {
		byLong[e.longName] = e.code
		bySimple[e.simpleName] = e.code
		byCode[e.code] = e.simpleName
	}
}

// containsChinese mirrors RegionEnum.containsChineseByRegex.
func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// CodeByName returns the OCI region code for a friendly (Chinese) name.
// Parity with RegionEnum.getRegionCode: if the input has no Chinese it is
// assumed to already be a code and returned as-is; otherwise the long or
// simple name is resolved; unresolved input is returned unchanged.
func CodeByName(name string) string {
	if name == "" {
		return ""
	}
	if !containsChinese(name) {
		return name
	}
	if c, ok := byLong[name]; ok {
		return c
	}
	if c, ok := bySimple[name]; ok {
		return c
	}
	return name
}

// NameByCode returns the simple (Chinese) name for a region code, or the
// input unchanged if unknown. Parity with RegionEnum.getNameByCode.
func NameByCode(code string) string {
	if code == "" {
		return ""
	}
	if n, ok := byCode[code]; ok {
		return n
	}
	return code
}

// All returns every (code, simpleName) pair for UI dropdowns.
func All() []Region {
	out := make([]Region, 0, len(entries))
	for _, e := range entries {
		out = append(out, Region{Code: e.code, Name: e.simpleName})
	}
	return out
}

type Region struct {
	Code string
	Name string
}
