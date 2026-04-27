package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"smarttraffic/internal/models"
	"smarttraffic/internal/repository"
	"smarttraffic/internal/services"

	qrcodepkg "smarttraffic/pkg/qrcode"
)

type PeerHandler struct {
	wgSvc  *services.WireGuardService
	sbSvc  *services.SingBoxService
	logger *slog.Logger
}

func NewPeerHandler(wgSvc *services.WireGuardService, sbSvc *services.SingBoxService, logger *slog.Logger) *PeerHandler {
	return &PeerHandler{wgSvc: wgSvc, sbSvc: sbSvc, logger: logger}
}

func (h *PeerHandler) List(w http.ResponseWriter, r *http.Request) {
	peers, err := h.wgSvc.ListPeers(r.Context())
	if err != nil {
		h.logger.Error("ошибка получения списка клиентов", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}
	if peers == nil {
		peers = []*models.Peer{}
	}
	JSON(w, http.StatusOK, peers)
}

func (h *PeerHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.PeerCreateRequest
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if errs := req.Validate(); len(errs) > 0 {
		JSON(w, http.StatusBadRequest, map[string]interface{}{"errors": errs})
		return
	}

	peer, err := h.wgSvc.CreatePeer(r.Context(), &req)
	if err != nil {
		h.logger.Error("ошибка создания клиента", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if err := h.sbSvc.WriteConfigAndRestart(r.Context()); err != nil {
		h.logger.Error("ошибка перезапуска sing-box после создания клиента", "id", peer.ID, "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "клиент создан, но не удалось перезапустить sing-box")
		return
	}

	JSON(w, http.StatusCreated, peer)
}

func (h *PeerHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	peer, err := h.wgSvc.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		h.logger.Error("ошибка получения клиента", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	peer.PrivateKey = ""
	JSON(w, http.StatusOK, peer)
}

func (h *PeerHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	if err := h.wgSvc.DeletePeer(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		h.logger.Error("ошибка удаления клиента", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if err := h.sbSvc.WriteConfigAndRestart(r.Context()); err != nil {
		h.logger.Warn("не удалось перезапустить sing-box после удаления клиента", "id", id, "error", err)
	}

	JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *PeerHandler) DownloadConfig(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	peer, err := h.wgSvc.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	config := h.wgSvc.GenerateClientConfig(peer)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+peer.Name+".json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config))
}

func (h *PeerHandler) GetQRCode(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	peer, err := h.wgSvc.GetPeer(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	config, err := h.wgSvc.GenerateClientConfigCompact(peer)
	if err != nil {
		h.logger.Error("ошибка генерации конфига для QR", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "ошибка генерации конфигурации")
		return
	}

	png, err := qrcodepkg.GeneratePNG(config, 512)
	if err != nil {
		h.logger.Error("ошибка генерации QR-кода", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "ошибка генерации QR-кода")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	w.Write(png)
}

func (h *PeerHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	stats, err := h.wgSvc.GetPeerStats(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	JSON(w, http.StatusOK, stats)
}

func (h *PeerHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get(":id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		ErrorJSON(w, http.StatusBadRequest, "id не указан")
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		ErrorJSON(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	if err := h.wgSvc.TogglePeer(r.Context(), id, req.Active); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			ErrorJSON(w, http.StatusNotFound, "клиент не найден")
			return
		}
		h.logger.Error("ошибка переключения клиента", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
		return
	}

	if err := h.sbSvc.WriteConfigAndRestart(r.Context()); err != nil {
		h.logger.Warn("не удалось перезапустить sing-box после toggle клиента", "id", id, "error", err)
	}

	JSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *PeerHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if err := h.sbSvc.WriteConfigAndReload(r.Context()); err != nil {
		h.logger.Error("ошибка синхронизации конфигурации sing-box", "error", err)
		ErrorJSON(w, http.StatusInternalServerError, "ошибка синхронизации")
		return
	}
	JSON(w, http.StatusOK, map[string]string{"status": "synced"})
}
