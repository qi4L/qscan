package pocScan

import (
	"Qscan/app"
	"Qscan/core/pocScan/lib"
	"compress/gzip"
	"crypto/tls"
	"errors"
	"fmt"
	"golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func WebTitle(info *app.HostInfo) error {
	err, CheckData := GOWebTitle(info)
	info.Infostr = InfoCheck(info.Url, &CheckData)
	WebScan(info)
	return err
}

func GOWebTitle(info *app.HostInfo) (err error, CheckData []CheckDatas) {
	if ip := net.ParseIP(info.Host); ip != nil {
		if ip.To4() != nil {
			if info.Url == "" {
				switch info.Ports {
				case "80":
					info.Url = fmt.Sprintf("http://%s", info.Host)
				case "443":
					info.Url = fmt.Sprintf("https://%s", info.Host)
				default:
					host := fmt.Sprintf("%s:%s", info.Host, info.Ports)
					protocol := GetProtocol(host, int64(app.Args.Timeout))
					info.Url = fmt.Sprintf("%s://%s:%s", protocol, info.Host, info.Ports)
				}
			} else {
				if !strings.Contains(info.Url, "://") {
					host := strings.Split(info.Url, "/")[0]
					protocol := GetProtocol(host, int64(app.Args.Timeout))
					info.Url = fmt.Sprintf("%s://%s", protocol, info.Url)
				}
			}
		} else if ip.To16() != nil {
			if info.Url == "" {
				switch info.Ports {
				case "80":
					info.Url = fmt.Sprintf("http://[%s]", info.Host)
				case "443":
					info.Url = fmt.Sprintf("https://[%s]", info.Host)
				default:
					host := fmt.Sprintf("[%s]:%s", info.Host, info.Ports)
					protocol := GetProtocol(host, int64(app.Args.Timeout))
					info.Url = fmt.Sprintf("%s://[%s]:%s", protocol, info.Host, info.Ports)
				}
			} else {
				if !strings.Contains(info.Url, "://") {
					host := strings.Split(info.Url, "/")[0]
					protocol := GetProtocol(host, int64(app.Args.Timeout))
					info.Url = fmt.Sprintf("%s://%s", protocol, info.Url)
				}
			}
		}
	}
	err, result, CheckData := geturl(info, 1, CheckData)
	if err != nil && !strings.Contains(err.Error(), "EOF") {
		return
	}

	//有跳转
	if strings.Contains(result, "://") {
		info.Url = result
		err, result, CheckData = geturl(info, 3, CheckData)
		if err != nil {
			return
		}
	}

	if result == "https" && !strings.HasPrefix(info.Url, "https://") {
		info.Url = strings.Replace(info.Url, "http://", "https://", 1)
		err, result, CheckData = geturl(info, 1, CheckData)
		//有跳转
		if strings.Contains(result, "://") {
			info.Url = result
			err, _, CheckData = geturl(info, 3, CheckData)
			if err != nil {
				return
			}
		}
	}
	//是否访问图标
	//err, _, CheckData = geturl(info, 2, CheckData)
	if err != nil {
		return
	}
	return
}

func geturl(info *app.HostInfo, flag int, CheckData []CheckDatas) (error, string, []CheckDatas) {
	//flag 1 first try
	//flag 2 /favicon.ico
	//flag 3 302
	//flag 4 400 -> https

	Url := info.Url
	if flag == 2 {
		URL, err := url.Parse(Url)
		if err == nil {
			Url = fmt.Sprintf("%s://%s/favicon.ico", URL.Scheme, URL.Host)
		} else {
			Url += "/favicon.ico"
		}
	}
	req, err := http.NewRequest("GET", Url, nil)
	if err != nil {
		return err, "", CheckData
	}
	req.Header.Set("User-agent", app.UserAgent)
	req.Header.Set("Accept", app.Accept)
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	if app.Args.Cookie != "" {
		req.Header.Set("Cookie", app.Args.Cookie)
	}
	//if common.Pocinfo.Cookie != "" {
	//	req.Header.Set("Cookie", "rememberMe=1;"+common.Pocinfo.Cookie)
	//} else {
	//	req.Header.Set("Cookie", "rememberMe=1")
	//}
	req.Header.Set("Connection", "close")
	var client *http.Client
	if flag == 1 {
		client = lib.ClientNoRedirect
	} else {
		client = lib.Client
	}

	resp, err := client.Do(req)
	if err != nil {
		return err, "https", CheckData
	}

	defer resp.Body.Close()
	var title string
	body, err := getRespBody(resp)
	if err != nil {
		return err, "https", CheckData
	}
	CheckData = append(CheckData, CheckDatas{body, fmt.Sprintf("%s", resp.Header)})
	var reurl string
	if flag != 2 {
		if !utf8.Valid(body) {
			body, _ = simplifiedchinese.GBK.NewDecoder().Bytes(body)
		}
		title = gettitle(body)
		length := resp.Header.Get("Content-Length")
		if length == "" {
			length = fmt.Sprintf("%v", len(body))
		}
		redirURL, err1 := resp.Location()
		if err1 == nil {
			reurl = redirURL.String()
		}
		result := fmt.Sprintf("[*] WebTitle %-25v code:%-3v len:%-6v title:%v", resp.Request.URL, resp.StatusCode, length, title)
		if reurl != "" {
			result += fmt.Sprintf(" 跳转url: %s", reurl)
		}
		//fmt.Println(result)
	}
	if reurl != "" {
		return nil, reurl, CheckData
	}
	if resp.StatusCode == 400 && !strings.HasPrefix(info.Url, "https") {
		return nil, "https", CheckData
	}
	return nil, "", CheckData
}

func getRespBody(oResp *http.Response) ([]byte, error) {
	var body []byte
	if oResp.Header.Get("Content-Encoding") == "gzip" {
		gr, err := gzip.NewReader(oResp.Body)
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		for {
			buf := make([]byte, 1024)
			n, err := gr.Read(buf)
			if err != nil && err != io.EOF {
				return nil, err
			}
			if n == 0 {
				break
			}
			body = append(body, buf...)
		}
	} else {
		raw, err := io.ReadAll(oResp.Body)
		if err != nil {
			return nil, err
		}
		body = raw
	}
	return body, nil
}

func gettitle(body []byte) (title string) {
	re := regexp.MustCompile("(?ims)<title>(.*?)</title>")
	find := re.FindSubmatch(body)
	if len(find) > 1 {
		title = string(find[1])
		title = strings.TrimSpace(title)
		title = strings.Replace(title, "\n", "", -1)
		title = strings.Replace(title, "\r", "", -1)
		title = strings.Replace(title, "&nbsp;", " ", -1)
		if len(title) > 100 {
			title = title[:100]
		}
		if title == "" {
			title = "\"\"" //空格
		}
	} else {
		title = "None" //没有title
	}
	return
}

func GetProtocol(host string, Timeout int64) (protocol string) {
	protocol = "http"
	//如果端口是80或443,跳过Protocol判断
	if strings.HasSuffix(host, ":80") || !strings.Contains(host, ":") {
		return
	} else if strings.HasSuffix(host, ":443") {
		protocol = "https"
		return
	}

	socksconn, err := WrapperTcpWithTimeout("tcp", host, time.Duration(Timeout)*time.Second)
	if err != nil {
		return
	}
	conn := tls.Client(socksconn, &tls.Config{MinVersion: tls.VersionTLS10, InsecureSkipVerify: true})
	defer func() {
		if conn != nil {
			defer func() {
				if err := recover(); err != nil {
					//common2.LogError(err)
				}
			}()
			conn.Close()
		}
	}()
	conn.SetDeadline(time.Now().Add(time.Duration(Timeout) * time.Second))
	err = conn.Handshake()
	if err == nil || strings.Contains(err.Error(), "handshake failure") {
		protocol = "https"
	}
	return protocol
}

func WrapperTcpWithTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	return WrapperTCP(network, address, d)
}

func WrapperTCP(network, address string, forward *net.Dialer) (net.Conn, error) {
	//get conn
	var conn net.Conn
	if app.Args.Proxy == "" {
		var err error
		conn, err = forward.Dial(network, address)
		if err != nil {
			return nil, err
		}
	} else {
		dailer, err := Socks5Dailer(forward)
		if err != nil {
			return nil, err
		}
		conn, err = dailer.Dial(network, address)
		if err != nil {
			return nil, err
		}
	}
	return conn, nil

}

func Socks5Dailer(forward *net.Dialer) (proxy.Dialer, error) {
	u, err := url.Parse(app.Args.Proxy)
	if err != nil {
		return nil, err
	}
	if strings.ToLower(u.Scheme) != "socks5" {
		return nil, errors.New("Only support socks5")
	}
	address := u.Host
	var auth proxy.Auth
	var dailer proxy.Dialer
	if u.User.String() != "" {
		auth = proxy.Auth{}
		auth.User = u.User.Username()
		password, _ := u.User.Password()
		auth.Password = password
		dailer, err = proxy.SOCKS5("tcp", address, &auth, forward)
	} else {
		dailer, err = proxy.SOCKS5("tcp", address, nil, forward)
	}

	if err != nil {
		return nil, err
	}
	return dailer, nil
}
