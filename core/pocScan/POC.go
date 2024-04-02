package pocScan

import (
	"KscanPro/app"
	"KscanPro/core/pocScan/lib"
	"embed"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

//go:embed pocs
var Pocs embed.FS
var once sync.Once
var AllPocs []*lib.Poc

func WebScan(info *app.HostInfo) {
	once.Do(initpoc)
	var pocinfo = app.PocInfo{}
	URL, _ := removeStandardPorts(info.Url)
	buf := strings.Split(URL, "/")
	pocinfo.Target = strings.Join(buf[:3], "/")

	if pocinfo.PocName != "" {
		Execute(pocinfo)
	} else {
		for _, infoStr := range info.Infostr {
			pocinfo.PocName = lib.CheckInfoPoc(infoStr)
			Execute(pocinfo)
		}
	}
}

func Execute(PocInfo app.PocInfo) {
	req, err := http.NewRequest("GET", PocInfo.Target, nil)
	if err != nil {
		//errlog := fmt.Sprintf("[-] webpocinit %v %v", PocInfo.Target, err)
		//common2.LogError(errlog)
		return
	}
	req.Header.Set("User-agent", app.UserAgent)
	req.Header.Set("Accept", app.Accept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if app.Setting.Cookie != "" {
		req.Header.Set("Cookie", app.Setting.Cookie)
	}
	pocs := filterPoc(PocInfo.PocName)
	lib.CheckMultiPoc(req, pocs, app.Setting.PocNum)
}

func initpoc() {
	entries, err := Pocs.ReadDir("pocs")
	if err != nil {
		fmt.Printf("[-] init poc error: %v", err)
		return
	}
	for _, one := range entries {
		path := one.Name()
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			if poc, _ := lib.LoadPoc(path, Pocs); poc != nil {
				AllPocs = append(AllPocs, poc)
			}
		}
	}

}

func filterPoc(pocname string) (pocs []*lib.Poc) {
	if pocname == "" {
		return AllPocs
	}
	for _, poc := range AllPocs {
		if strings.Contains(poc.Name, pocname) {
			pocs = append(pocs, poc)
		}
	}
	return
}

// 从URL字符串中移除端口80和443
func removeStandardPorts(rawURL string) (string, error) {
	// 解析URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// 获取端口部分
	host := u.Host
	colonIndex := strings.LastIndex(host, ":")
	if colonIndex != -1 {
		port := host[colonIndex+1:]
		if port == "80" || port == "443" {
			// 移除端口
			u.Host = host[:colonIndex]
		}
	}

	// 重建URL字符串，不包括默认端口
	return u.String(), nil
}
