package webui

import (
	. "github.com/Aptomi/aptomi/pkg/slinga/db"
	"github.com/Aptomi/aptomi/pkg/slinga/engine"
	. "github.com/Aptomi/aptomi/pkg/slinga/language"
	"github.com/Aptomi/aptomi/pkg/slinga/webui/visibility"
	"github.com/julienschmidt/httprouter"
	"net/http"
)

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./webui/favicon.ico")
}

func endpointsHandler(w http.ResponseWriter, r *http.Request) {
	// Load the previous usage state
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	endpoints := visibility.Endpoints(getLoggedInUserID(r), state)

	writeJSON(w, endpoints)
}

func detailViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	userID := getLoggedInUserID(r)
	view := visibility.NewDetails(userID, state)
	writeJSON(w, view)
}

func consumerViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	userID := r.URL.Query().Get("userId")
	dependencyID := r.URL.Query().Get("dependencyId")
	view := visibility.NewConsumerView(userID, dependencyID, state)
	writeJSON(w, view.GetData())
}

func serviceViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	serviceName := r.URL.Query().Get("serviceName")
	view := visibility.NewServiceView(serviceName, state)
	writeJSON(w, view.GetData())
}

func globalOpsViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	userID := r.URL.Query().Get("userId")
	dependencyID := r.URL.Query().Get("dependencyId")
	view := visibility.NewConsumerView(userID, dependencyID, state)
	writeJSON(w, view.GetData())
}

func objectViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	objectID := r.URL.Query().Get("id")
	view := visibility.NewObjectView(objectID, state)
	writeJSON(w, view.GetData())
}

func summaryViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	state := engine.LoadServiceUsageState(userLoader)
	userID := getLoggedInUserID(r)
	view := visibility.NewSummaryView(userID, state)
	writeJSON(w, view.GetData())
}

func timelineViewHandler(w http.ResponseWriter, r *http.Request) {
	userLoader := NewAptomiUserLoader()
	states := engine.LoadServiceUsageStatesAll(userLoader)
	userID := getLoggedInUserID(r)
	view := visibility.NewTimelineView(userID, states)
	writeJSON(w, view.GetData())
}

// Serve starts http server on specified address that serves Aptomi API and WebUI
func Serve(r *httprouter.Router) {
	r.HandlerFunc(http.MethodGet, "/favicon.ico", faviconHandler)

	// redirect from "/" to "/ui/"
	r.Handler(http.MethodGet, "/", http.RedirectHandler("/ui/", http.StatusTemporaryRedirect))

	// serve all files from "webui" folder and require auth for everything except login.html
	r.Handler(http.MethodGet, "/ui/", publicFilesHandler("/ui/", http.Dir("./webui/")))
	r.Handler(http.MethodGet, "/run/", runFilesHandler("/run/", http.Dir(GetAptomiBaseDir())))

	// serve all API endpoints at /api/ path and require auth
	r.Handler(http.MethodGet, "/api/endpoints", requireAuth(endpointsHandler))
	r.Handler(http.MethodGet, "/api/details", requireAuth(detailViewHandler))
	r.Handler(http.MethodGet, "/api/service-view", requireAuth(serviceViewHandler))
	r.Handler(http.MethodGet, "/api/consumer-view", requireAuth(consumerViewHandler))
	r.Handler(http.MethodGet, "/api/globalops-view", requireAuth(globalOpsViewHandler))
	r.Handler(http.MethodGet, "/api/object-view", requireAuth(objectViewHandler))
	r.Handler(http.MethodGet, "/api/summary-view", requireAuth(summaryViewHandler))
	r.Handler(http.MethodGet, "/api/timeline-view", requireAuth(timelineViewHandler))

	// serve login/logout api without auth
	r.HandlerFunc(http.MethodGet, "/api/login", loginHandler)
	r.HandlerFunc(http.MethodGet, "/api/logout", logoutHandler)
}
