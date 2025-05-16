package controller

import (
	"io/fs"
	"net/http"
	"net/url"

	"github.com/computer-technology-team/distributed-kvstore/web"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type adminServer struct {
	controller *Controller
	router     chi.Router
	renderer   web.TemplateRenderer
	staticFS   fs.FS
}

// AdminServer provides an interface for admin UI functionality
type AdminServer interface {
	// Router returns the Chi router for the admin server
	Router() chi.Router

	// SetupRoutes configures all routes for the admin UI
	SetupRoutes()

	// Dashboard renders the main admin dashboard
	Dashboard(w http.ResponseWriter, r *http.Request)

	// PartitionsList renders the partitions management page
	PartitionsList(w http.ResponseWriter, r *http.Request)

	// PartitionDetail renders details for a specific partition
	PartitionDetail(w http.ResponseWriter, r *http.Request)

	// AddPartition handles the creation of a new partition
	AddPartition(w http.ResponseWriter, r *http.Request)

	// RemovePartition handles the removal of a partition
	RemovePartition(w http.ResponseWriter, r *http.Request)

	// NodesList renders the nodes management page
	NodesList(w http.ResponseWriter, r *http.Request)

	// AddNode handles the addition of a new node
	AddNode(w http.ResponseWriter, r *http.Request)

	// RemoveNode handles the removal of a node
	RemoveNode(w http.ResponseWriter, r *http.Request)

	// SystemStats renders system statistics page
	SystemStats(w http.ResponseWriter, r *http.Request)
}

// NewAdminServer creates a new AdminServer instance
func NewAdminServer(controller *Controller) (AdminServer, error) {
	renderer, err := web.NewTemplateRenderer()
	if err != nil {
		return nil, err
	}

	staticFS, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		return nil, err
	}

	server := &adminServer{
		controller: controller,
		router:     chi.NewRouter(),
		renderer:   renderer,
		staticFS:   staticFS,
	}

	// Setup routes immediately
	server.SetupRoutes()

	return server, nil
}

// Router returns the Chi router for the admin server
func (a *adminServer) Router() chi.Router {
	return a.router
}

// SetupRoutes configures all routes for the admin UI
func (a *adminServer) SetupRoutes() {
	// Serve static files
	fileServer := http.FileServer(http.FS(a.staticFS))
	a.router.Handle("/static/*", http.StripPrefix("/static", fileServer))

	// Admin UI routes grouped by functionality
	a.router.Route("/", func(r chi.Router) {
		// Dashboard
		r.Get("/", a.Dashboard)

		// Partitions management
		r.Route("/partitions", func(r chi.Router) {
			r.Get("/", a.PartitionsList)
			r.Get("/{id}", a.PartitionDetail)
			r.Post("/add", a.AddPartition)
			r.Post("/remove", a.RemovePartition)
		})

		// Nodes management
		r.Route("/nodes", func(r chi.Router) {
			r.Get("/", a.NodesList)
			r.Post("/add", a.AddNode)
			r.Post("/remove", a.RemoveNode)
		})

		// System statistics
		r.Get("/stats", a.SystemStats)
	})
}

// Dashboard renders the main admin dashboard
func (a *adminServer) Dashboard(w http.ResponseWriter, r *http.Request) {
	state := a.controller.GetState()

	data := map[string]any{
		"Title":        "Admin Dashboard",
		"Partitions":   state.Partitions,
		"VirtualNodes": state.VirtualNodes,
	}

	a.renderTemplate(w, "dashboard.html", data)
}

// PartitionsList renders the partitions management page
func (a *adminServer) PartitionsList(w http.ResponseWriter, r *http.Request) {
	state := a.controller.GetState()

	data := map[string]any{
		"Title":      "Partitions Management",
		"Partitions": state.Partitions,
	}

	a.renderTemplate(w, "partitions.html", data)
}

// PartitionDetail renders details for a specific partition
func (a *adminServer) PartitionDetail(w http.ResponseWriter, r *http.Request) {
	partitionID := chi.URLParam(r, "id")

	state := a.controller.GetState()

	partition, exists := state.Partitions[partitionID]
	if !exists {
		http.Error(w, "Partition not found", http.StatusNotFound)
		return
	}

	data := map[string]any{
		"Title":     "Partition Details",
		"Partition": partition,
	}

	a.renderTemplate(w, "partition_detail.html", data)
}

// AddPartition handles the creation of a new partition
func (a *adminServer) AddPartition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	partitionID := r.FormValue("partition_id")
	if partitionID == "" {
		http.Error(w, "Partition ID is required", http.StatusBadRequest)
		return
	}

	// Add the partition to the controller
	err = a.controller.AddPartition(partitionID)
	if err != nil {
		http.Error(w, "Failed to add partition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to the partitions list
	http.Redirect(w, r, "/partitions", http.StatusSeeOther)
}

// RemovePartition handles the removal of a partition
func (a *adminServer) RemovePartition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	partitionID := r.FormValue("partition_id")
	if partitionID == "" {
		http.Error(w, "Partition ID is required", http.StatusBadRequest)
		return
	}

	// Remove the partition from the controller
	err = a.controller.RemovePartition(partitionID)
	if err != nil {
		http.Error(w, "Failed to remove partition: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to the partitions list
	http.Redirect(w, r, "/partitions", http.StatusSeeOther)
}

// NodesList renders the nodes management page
func (a *adminServer) NodesList(w http.ResponseWriter, r *http.Request) {
	state := a.controller.GetState()

	data := map[string]any{
		"Title":             "Nodes Management",
		"Nodes":             state.Nodes,
		"UnRegisteredNodes": state.UnRegisteredNodes,
	}

	a.renderTemplate(w, "nodes.html", data)
}

// AddNode handles the addition of a new node
func (a *adminServer) AddNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Check if we're registering an unregistered node
	unregisteredNodeID := r.FormValue("unregistered_node_id")
	if unregisteredNodeID != "" {
		// Register an existing unregistered node
		_, err = a.controller.RegisterNode(unregisteredNodeID)
		if err != nil {
			http.Error(w, "Failed to register node: "+err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		// Legacy path for manually adding a node (should not be used anymore)
		nodeID := uuid.New()
		nodeAddress := r.FormValue("node_address")

		if nodeAddress == "" {
			http.Error(w, "address is required", http.StatusBadRequest)
			return
		}

		_, err = url.Parse("http://" + nodeAddress)
		if err != nil {
			http.Error(w, "could not parse address", http.StatusBadRequest)
		}

		// Add the node to the controller
		err = a.controller.AddNode(nodeID, nodeAddress)
		if err != nil {
			http.Error(w, "Failed to add node: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to the nodes list
	http.Redirect(w, r, "/nodes", http.StatusSeeOther)
}

// RemoveNode handles the removal of a node
func (a *adminServer) RemoveNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	nodeID := r.FormValue("node_id")
	if nodeID == "" {
		http.Error(w, "Node ID is required", http.StatusBadRequest)
		return
	}

	// Remove the node from the controller
	err = a.controller.RemoveNode(nodeID)
	if err != nil {
		http.Error(w, "Failed to remove node: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to the nodes list
	http.Redirect(w, r, "/nodes", http.StatusSeeOther)
}

// SystemStats renders system statistics page
func (a *adminServer) SystemStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]any{
		"TotalRequests": 0,
		"Uptime":        a.controller.GetUptime().String(),
		// Add more stats as needed
	}

	data := map[string]any{
		"Title": "System Statistics",
		"Stats": stats,
	}

	a.renderTemplate(w, "stats.html", data)
}

// renderTemplate is a helper function to render templates with error handling
func (a *adminServer) renderTemplate(w http.ResponseWriter, tmpl string, data any) {
	err := a.renderer.Render(w, tmpl, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
