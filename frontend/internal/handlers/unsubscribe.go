package handlers

import (
	"log"
	"net/http"
)

// Unsubscribe is a public, unauthenticated pass-through for Gmail's one-click
// unsubscribe flow (RFC 8058): mailbox providers POST directly to this URL
// with no session, so there's nothing here to authenticate against —
// backend-api's own HMAC signature check over the family/user params is the
// real gate. This handler just forwards them and relays the result. GET is
// read-only and asks for confirmation before a manual POST.
func (h *Handlers) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	family := r.URL.Query().Get("family")
	user := r.URL.Query().Get("user")
	sig := r.URL.Query().Get("sig")
	if family == "" || user == "" || sig == "" {
		http.Error(w, "invalid unsubscribe link", http.StatusBadRequest)
		return
	}

	if r.Method == http.MethodGet {
		writeUnsubscribePage(w, false)
		return
	}

	manual := r.FormValue("manual") == "1"
	if err := h.Backend.Unsubscribe(r.Context(), family, user, sig); err != nil {
		log.Printf("unsubscribe: %v", err)
		writeBackendEventError(w, err, "failed to unsubscribe; please try again")
		return
	}

	if manual {
		writeUnsubscribePage(w, true)
		return
	}

	// Mailbox providers' one-click flow expects a bare success response.
	w.WriteHeader(http.StatusOK)
}

func writeUnsubscribePage(w http.ResponseWriter, complete bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if complete {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Unsubscribed</title></head>
<body style="margin:0; padding:80px 20px; background:#FAF6F1; font-family:Arial, Helvetica, sans-serif; color:#3A332C; text-align:center;">
  <p style="font-size:16px;">You&#39;ve been unsubscribed from Yauli report emails.</p>
</body>
</html>`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>Unsubscribe</title></head>
<body style="margin:0; padding:80px 20px; background:#FAF6F1; font-family:Arial, Helvetica, sans-serif; color:#3A332C; text-align:center;">
  <p style="font-size:16px;">Unsubscribe from Yauli report emails?</p>
  <form method="post">
    <button type="submit" name="manual" value="1" style="padding:12px 20px; border:0; border-radius:8px; background:#5FBCB0; color:white; font-size:16px; cursor:pointer;">Unsubscribe</button>
  </form>
</body>
</html>`))
}
