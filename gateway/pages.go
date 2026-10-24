package main

import (
	"html/template"
	"net/http"
	"net/url"
)

const pairPageSource = `<!doctype html>
<html lang="vi"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta http-equiv="refresh" content="4">
<title>FreeTV</title>
<style>
body{font-family:system-ui,sans-serif;max-width:560px;margin:32px auto;padding:0 16px;
background:#111;color:#eee}
.row{display:flex;justify-content:space-between;align-items:center;padding:14px 0;
border-top:1px solid #333}
a.button{padding:8px 14px;border-radius:8px;text-decoration:none;color:#eee;
border:1px solid #666;margin-left:8px}
a.primary{background:#4f8cff;border-color:#4f8cff;color:#fff}
.code{font-size:20px;letter-spacing:2px;font-weight:600}
.muted{opacity:.7}
</style></head><body>
<h1>FreeTV</h1>
<p class="muted">Chỉ duyệt khi mã trùng với mã đang hiện trên TV.
Bạn sẽ đăng nhập Home Assistant để xác nhận.</p>
{{if .Notice}}<p><b>{{.Notice}}</b></p>{{end}}
{{range .Pending}}
<div class="row"><div><div>{{.Name}}</div><div class="code">{{.Code}}</div></div>
<div><a class="button primary" href="/pair/approve?request_id={{.ID}}">Duyệt</a>
<a class="button" href="/pair/reject?request_id={{.ID}}">Từ chối</a></div></div>
{{else}}<p class="muted">Không có TV nào đang chờ.</p>{{end}}
</body></html>`

var pairPage = template.Must(template.New("pair").Parse(pairPageSource))

type pairPageData struct {
	Pending []PairingRequest
	Notice  string
}

func (gateway *Gateway) renderPairPage(writer http.ResponseWriter, notice string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	_ = pairPage.Execute(writer, pairPageData{Pending: gateway.pairing.Pending(), Notice: notice})
}

func (gateway *Gateway) handlePairPage(writer http.ResponseWriter, _ *http.Request) {
	gateway.renderPairPage(writer, "")
}

func (gateway *Gateway) handleApprove(writer http.ResponseWriter, request *http.Request) {
	nonce, beginErr := gateway.pairing.BeginApproval(request.URL.Query().Get("request_id"))
	if beginErr != nil {
		gateway.renderPairPage(writer, beginErr.Error())
		return
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     approvalCookie,
		Value:    nonce,
		Path:     callbackPath,
		MaxAge:   approvalCookieAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	base := gateway.baseURL(request)
	target := gateway.homeAssistant.AuthorizeURL(base, base+callbackPath, nonce)
	http.Redirect(writer, request, target, http.StatusFound)
}

func (gateway *Gateway) handleReject(writer http.ResponseWriter, request *http.Request) {
	notice := "Đã từ chối"
	requestID := request.URL.Query().Get("request_id")
	if rejectErr := gateway.pairing.Reject(requestID); rejectErr != nil {
		notice = rejectErr.Error()
	}
	gateway.renderPairPage(writer, notice)
}

func (gateway *Gateway) handleCallback(writer http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	nonce := query.Get("state")
	cookie, cookieErr := request.Cookie(approvalCookie)
	if cookieErr != nil || cookie.Value != nonce || query.Get("code") == "" {
		gateway.renderPairPage(writer, "Phiên duyệt không hợp lệ, hãy thử lại")
		return
	}

	base := gateway.baseURL(request)
	grant, exchangeErr := gateway.homeAssistant.ExchangeCode(
		request.Context(), query.Get("code"), base,
	)
	if exchangeErr != nil {
		gateway.renderPairPage(writer, "Home Assistant từ chối đăng nhập")
		return
	}

	gateway.finishApproval(writer, request, nonce, base, grant)
}

func (gateway *Gateway) finishApproval(
	writer http.ResponseWriter,
	request *http.Request,
	nonce, base string,
	grant TokenGrant,
) {
	pending, nameErr := gateway.pairing.PendingNameFor(nonce)
	if nameErr != nil {
		gateway.homeAssistant.Revoke(request.Context(), grant.RefreshToken)
		gateway.renderPairPage(writer, nameErr.Error())
		return
	}

	token, addErr := gateway.sessions.Add(pending, base, grant.RefreshToken)
	if addErr != nil {
		gateway.homeAssistant.Revoke(request.Context(), grant.RefreshToken)
		gateway.renderPairPage(writer, "Không lưu được phiên")
		return
	}

	if _, approveErr := gateway.pairing.CompleteApproval(nonce, token); approveErr != nil {
		_, _ = gateway.sessions.Remove(hashToken(token))
		gateway.homeAssistant.Revoke(request.Context(), grant.RefreshToken)
		gateway.renderPairPage(writer, approveErr.Error())
		return
	}

	redirect := url.URL{Path: pairPagePath}
	http.Redirect(writer, request, redirect.String(), http.StatusSeeOther)
}
