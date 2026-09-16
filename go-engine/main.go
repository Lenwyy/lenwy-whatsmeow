// Lenwy Was Here: Lenwy Whatsmeow Engine
// Disclaimer: This code is provided as-is and may not be suitable for production use. Use at your own risk.
// Under the MIT License (MIT). Copyright (c) 2024 Lenwy. All rights reserved.

// Thanks to the following libraries:
// - github.com/mattn/go-sqlite3
// - go.mau.fi/whatsmeow

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type IPCEvent struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type IPCCommand struct {
	Action  string          `json:"action"`
	ID      string          `json:"id"`
	Payload json.RawMessage `json:"payload"`
}

type SendMessagePayload struct {
	JID          string `json:"jid"`
	Text         string `json:"text"`
	QuotedID     string `json:"quotedId"`
	QuotedSender string `json:"quotedSender"`
}

type SendMediaPayload struct {
	JID          string `json:"jid"`
	MediaType    string `json:"mediaType"`
	FilePath     string `json:"filePath"`
	Caption      string `json:"caption"`
	FileName     string `json:"fileName"`
	QuotedID     string `json:"quotedId"`
	QuotedSender string `json:"quotedSender"`
    GifPlayback  bool   `json:"gifPlayback"`
}

type DownloadMediaPayload struct {
	MessageID string `json:"messageId"`
	OutputDir string `json:"outputDir"`
}

type PairPhonePayload struct {
	Phone string `json:"phone"`
}

type GroupJIDPayload struct {
	JID string `json:"jid"`
}

type ProfilePicturePayload struct {
	JID         string `json:"jid"`
	Type        string `json:"type"`
	ExistingID  string `json:"existingId"`
	IsCommunity bool   `json:"isCommunity"`
	CommonGID   string `json:"commonGid"`
	InviteCode  string `json:"inviteCode"`
}

type UserInfoPayload struct {
	JIDs []string `json:"jids"`
}

type IsOnWhatsAppPayload struct {
	Phones []string `json:"phones"`
}

type GroupLinkPayload struct {
	Code string `json:"code"`
}

type PresencePayload struct {
	State string `json:"state"`
}

type ChatPresencePayload struct {
	JID   string `json:"jid"`
	State string `json:"state"`
	Media string `json:"media"`
}

type MarkReadPayload struct {
	IDs       []string `json:"ids"`
	Timestamp int64    `json:"timestamp"`
	Chat      string   `json:"chat"`
	Sender    string   `json:"sender"`
	Played    bool     `json:"played"`
}

type GroupParticipantsPayload struct {
	JID          string   `json:"jid"`
	Participants []string `json:"participants"`
	Action       string   `json:"action"`
}

type GroupSettingPayload struct {
	JID     string `json:"jid"`
	Setting string `json:"setting"`
}

var (
	msgCache      = make(map[string]*events.Message)
	msgCacheMutex sync.RWMutex
)

func sendIPC(event string, data interface{}) {
	payload, err := json.Marshal(IPCEvent{
		Event: event,
		Data:  data,
	})

	if err == nil {
		fmt.Println(string(payload))
	}
}

func extractMediaMessage(msg *waProto.Message) (*waProto.Message, string, string) {
	if msg == nil {
		return nil, "", ""
	}

	if img := msg.GetImageMessage(); img != nil {
		ext := ".jpg"

		if img.GetMimetype() == "image/png" {
			ext = ".png"
		}

		return msg, ext, "image"
	}

	if msg.GetVideoMessage() != nil {
		return msg, ".mp4", "video"
	}

	if msg.GetStickerMessage() != nil {
		return msg, ".webp", "sticker"
	}

	if aud := msg.GetAudioMessage(); aud != nil {
		ext := ".mp3"

		if strings.Contains(aud.GetMimetype(), "ogg") {
			ext = ".ogg"
		}

		return msg, ext, "sound"
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		ext := filepath.Ext(doc.GetFileName())

		if ext == "" {
			ext = ".bin"
		}

		return msg, ext, "document"
	}

	if extMsg := msg.GetExtendedTextMessage(); extMsg != nil &&
		extMsg.ContextInfo != nil &&
		extMsg.ContextInfo.QuotedMessage != nil {
		return extractMediaMessage(extMsg.ContextInfo.QuotedMessage)
	}

	return nil, "", ""
}

func parseMessageContent(msg *waProto.Message) (string, string) {
	if msg == nil {
		return "Chat", ""
	}

	if text := msg.GetConversation(); text != "" {
		return "Chat", text
	}

	if extText := msg.GetExtendedTextMessage(); extText != nil {
		return "Chat", extText.GetText()
	}

	if img := msg.GetImageMessage(); img != nil {
		if caption := img.GetCaption(); caption != "" {
			return "Image", caption
		}
		return "Image", "Mengirimkan Gambar"
	}

	if vid := msg.GetVideoMessage(); vid != nil {
		if caption := vid.GetCaption(); caption != "" {
			return "Video", caption
		}
		return "Video", "Mengirimkan Video"
	}

	if msg.GetStickerMessage() != nil {
		return "Sticker", "Mengirimkan Stiker"
	}

	if doc := msg.GetDocumentMessage(); doc != nil {
		if fileName := doc.GetFileName(); fileName != "" {
			return "Document", fileName
		}
		return "Document", "Mengirimkan Dokumen"
	}

	if msg.GetAudioMessage() != nil {
		return "Audio", "Mengirimkan Audio"
	}

	return "Chat", ""
}

func getContextInfo(msg *waProto.Message) *waProto.ContextInfo {
	if msg == nil {
		return nil
	}
	if x := msg.GetExtendedTextMessage(); x != nil {
		return x.GetContextInfo()
	}
	if x := msg.GetImageMessage(); x != nil {
		return x.GetContextInfo()
	}
	if x := msg.GetVideoMessage(); x != nil {
		return x.GetContextInfo()
	}
	if x := msg.GetAudioMessage(); x != nil {
		return x.GetContextInfo()
	}
	if x := msg.GetDocumentMessage(); x != nil {
		return x.GetContextInfo()
	}
	if x := msg.GetStickerMessage(); x != nil {
		return x.GetContextInfo()
	}
	return nil
}

func getQuotedMessage(msg *waProto.Message) (string, string, string) {
	ctxInfo := getContextInfo(msg)
	if ctxInfo == nil {
		return "", "", ""
	}

	quotedID := ctxInfo.GetStanzaID()
	quotedSender := ctxInfo.GetParticipant()
	quotedText := ""
	quotedType := ""

	if quoted := ctxInfo.GetQuotedMessage(); quoted != nil {
		quotedType, quotedText = parseMessageContent(quoted)
	}

	return quotedID, quotedSender, quotedType + "\x00" + quotedText
}

// Lenwy Disclaimer: This code is provided as-is and may not be suitable for production use. Use at your own risk.
func main() {
	ctx := context.Background()

	sessionName := "lenwy"

	if len(os.Args) > 1 && os.Args[1] != "" {
		sessionName = os.Args[1]
	}

	sessionDir := filepath.Join("../sessions", sessionName)

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		panic(err)
	}

	dbPath := filepath.Join(sessionDir, "whatsmeow.db")

	dbLog := waLog.Stdout("Database", "ERROR", true)

	container, err := sqlstore.New(
		ctx,
		"sqlite3",
		fmt.Sprintf("file:%s?_foreign_keys=on", dbPath),
		dbLog,
	)

	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)

	if err != nil {
		panic(err)
	}

	clientLog := waLog.Stdout("Client", "ERROR", true)

	client := whatsmeow.NewClient(deviceStore, clientLog)

	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {

		case *events.Connected:
			botJid := ""

			if client.Store.ID != nil {
				botJid = client.Store.ID.ToNonAD().String()
			}

			sendIPC("connection.update", map[string]string{
				"status": "open",
				"botJid": botJid,
			})

		case *events.Disconnected:
			sendIPC("connection.update", map[string]interface{}{
				"status": "connecting",
				"reason": "connection_lost",
			})

		case *events.LoggedOut:
			sendIPC("connection.update", map[string]interface{}{
				"status": "close",
				"reason": "logged_out",
			})

			container.Close()
			os.Exit(0)

		case *events.Picture:
			sendIPC("profilePicture.update", map[string]interface{}{
				"jid":       v.JID.String(),
				"author":    v.Author.String(),
				"timestamp": v.Timestamp.Unix(),
				"remove":    v.Remove,
				"pictureId": v.PictureID,
			})

		case *events.Presence:
			sendIPC("presence.update", v)

		case *events.Receipt:
			sendIPC("receipt.update", v)

		case *events.Message:
			msgCacheMutex.Lock()
			msgCache[v.Info.ID] = v
			msgCacheMutex.Unlock()

			msgType, body := parseMessageContent(v.Message)

			var quotedID string
			var quotedSender string
			var quotedText string
			var quotedType string

			if ctxInfo := getContextInfo(v.Message); ctxInfo != nil {
				quotedID = ctxInfo.GetStanzaID()
				quotedSender = ctxInfo.GetParticipant()

				if quoted := ctxInfo.GetQuotedMessage(); quoted != nil {
					quotedType, quotedText = parseMessageContent(quoted)
				}
			}

			if quotedID != "" && quotedText == "" {
				msgCacheMutex.RLock()
				quotedMsg := msgCache[quotedID]
				msgCacheMutex.RUnlock()

				if quotedMsg != nil {
					quotedType, quotedText = parseMessageContent(quotedMsg.Message)

					if quotedSender == "" {
						quotedSender = quotedMsg.Info.Sender.String()
					}
				}
			}

			botJid := ""

			if client.Store.ID != nil {
				botJid = client.Store.ID.ToNonAD().String()
			}

			// Prioritaskan PNJID (Phone Number JID)
			senderJID := v.Info.Sender.ToNonAD()

			sendIPC("messages.upsert", map[string]interface{}{
				"id":           v.Info.ID,
				"chat":         v.Info.Chat.String(),
				"sender":       senderJID.User,
				"senderJid":    senderJID.String(),
				"pushName":     v.Info.PushName,
				"isFromMe":     v.Info.IsFromMe,
				"timestamp":    v.Info.Timestamp.Unix(),
				"type":         msgType,
				"body":         body,
				"quotedId":     quotedID,
				"quotedSender": quotedSender,
				"quotedType":   quotedType,
				"quotedText":   quotedText,
				"botJid":       botJid,
			})
		}
	})

	err = client.Connect()

	if err != nil {
		sendIPC("error", map[string]string{
			"message": "Gagal connect: " + err.Error(),
		})

		container.Close()
		return
	}

	go func() {
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			line := scanner.Text()

			var cmd IPCCommand

			if err := json.Unmarshal([]byte(line), &cmd); err != nil {
				continue
			}

			switch cmd.Action {

			// Lenwy Was Here: Shutdown Engine
			case "shutdown":
				if client.IsConnected() {
					client.Disconnect()
				}

				container.Close()

				sendIPC("response", map[string]interface{}{
					"id":     cmd.ID,
					"status": "ok",
				})

				os.Exit(0)

			// Pairing Kode
			case "requestPairingCode":
				var p PairPhonePayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					if !client.IsConnected() {
						sendIPC("error", map[string]string{
							"message": "Client belum terhubung ke server WA",
						})
						continue
					}

					code, err := client.PairPhone(
						ctx,
						p.Phone,
						true,
						whatsmeow.PairClientChrome,
						"Chrome (Linux)",
					)

					if err == nil {
						sendIPC("pairing_code", map[string]string{
							"code": code,
						})
					} else {
						sendIPC("error", map[string]string{
							"message": "Gagal pair: " + err.Error(),
						})
					}
				}

			case "getProfilePicture":
				var p ProfilePicturePayload

				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{
						"id":     cmd.ID,
						"status": "error",
						"error":  err.Error(),
					})
					continue
				}

				if p.JID == "" {
					sendIPC("response", map[string]interface{}{
						"id":     cmd.ID,
						"status": "error",
						"error":  "JID tidak boleh kosong",
					})
					continue
				}

				targetJID, err := types.ParseJID(p.JID)
				if err != nil {
					sendIPC("response", map[string]interface{}{
						"id":     cmd.ID,
						"status": "error",
						"error":  "Invalid JID: " + err.Error(),
					})
					continue
				}

				params := &whatsmeow.GetProfilePictureParams{
					Preview:     strings.EqualFold(p.Type, "preview") || strings.EqualFold(p.Type, "thumbnail"),
					ExistingID:  p.ExistingID,
					IsCommunity: p.IsCommunity,
					InviteCode:  p.InviteCode,
				}

				if p.CommonGID != "" {
					commonGID, parseErr := types.ParseJID(p.CommonGID)
					if parseErr != nil {
						sendIPC("response", map[string]interface{}{
							"id": cmd.ID, "status": "error", "error": "Invalid commonGid: " + parseErr.Error(),
						})
						continue
					}
					params.CommonGID = commonGID
				}

				info, err := client.GetProfilePictureInfo(ctx, targetJID, params)

				if err != nil {
					sendIPC("response", map[string]interface{}{
						"id":     cmd.ID,
						"status": "error",
						"error":  err.Error(),
					})
					continue
				}

				sendIPC("response", map[string]interface{}{
					"id":     cmd.ID,
					"status": "ok",
					"resp":   info,
				})

			case "getUserInfo":
				var p UserInfoPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}

				jids := make([]types.JID, 0, len(p.JIDs))
				for _, rawJID := range p.JIDs {
					j, err := types.ParseJID(rawJID)
					if err != nil {
						sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": "Invalid JID: " + err.Error()})
						continue
					}

					j = j.ToNonAD()
					if j.Server == types.DefaultUserServer {
						jids = append(jids, j)
					} else {
						pn, err := client.Store.LIDs.GetPNForLID(ctx, j)
						if err == nil && !pn.IsEmpty() {
							jids = append(jids, pn.ToNonAD())
						}
					}
				}

				info, err := client.GetUserInfo(ctx, jids)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}

				result := make([]map[string]interface{}, 0, len(jids))
				for _, jid := range jids {
					user, ok := info[jid]
					if !ok {
						continue
					}

					contact, _ := client.Store.Contacts.GetContact(ctx, jid)
					name := contact.FullName
					if name == "" {
						name = contact.FirstName
					}
					if name == "" {
						name = contact.PushName
					}
					if name == "" {
						name = contact.BusinessName
					}

					result = append(result, map[string]interface{}{
						"jid_pn":  jid.String(),
						"jid_lid": user.LID.String(),
						"name":    name,
						"info":    user.Status,
					})
				}

				sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": result})

			case "isOnWhatsApp":
				var p IsOnWhatsAppPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				info, err := client.IsOnWhatsApp(ctx, p.Phones)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": info})
				}

			case "getJoinedGroups":
				info, err := client.GetJoinedGroups(ctx)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": info})
				}

			case "getGroupInfoFromLink":
				var p GroupLinkPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				info, err := client.GetGroupInfoFromLink(ctx, p.Code)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": info})
				}

			case "joinGroupWithLink":
				var p GroupLinkPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				jid, err := client.JoinGroupWithLink(ctx, p.Code)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": jid.String()})
				}

			case "leaveGroup":
				var p GroupJIDPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				jid, err := types.ParseJID(p.JID)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				err = client.LeaveGroup(ctx, jid)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": true})
				}

			case "getBusinessProfile":
				var p GroupJIDPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				jid, err := types.ParseJID(p.JID)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				info, err := client.GetBusinessProfile(ctx, jid)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": info})
				}

			case "sendPresence":
				var p PresencePayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				state := types.PresenceAvailable
				if strings.EqualFold(p.State, "unavailable") || strings.EqualFold(p.State, "offline") {
					state = types.PresenceUnavailable
				}
				err := client.SendPresence(ctx, state)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": true})
				}

			case "sendChatPresence":
				var p ChatPresencePayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				jid, err := types.ParseJID(p.JID)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				state := types.ChatPresenceComposing
				if strings.EqualFold(p.State, "paused") || strings.EqualFold(p.State, "stop") {
					state = types.ChatPresencePaused
				}
				media := types.ChatPresenceMediaText
				if strings.EqualFold(p.Media, "audio") || strings.EqualFold(p.Media, "recording") {
					media = types.ChatPresenceMediaAudio
				}
				err = client.SendChatPresence(ctx, jid, state, media)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": true})
				}

			case "subscribePresence":
				var p GroupJIDPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				jid, err := types.ParseJID(p.JID)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				err = client.SubscribePresence(ctx, jid)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": true})
				}

			case "markRead":
				var p MarkReadPayload
				if err := json.Unmarshal(cmd.Payload, &p); err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				chat, err := types.ParseJID(p.Chat)
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
					continue
				}
				sender := types.JID{}
				if p.Sender != "" {
					sender, err = types.ParseJID(p.Sender)
					if err != nil {
						sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
						continue
					}
				}
				ts := time.Unix(p.Timestamp, 0)
				if p.Timestamp == 0 {
					ts = time.Now()
				}
				ids := make([]types.MessageID, 0, len(p.IDs))
				for _, id := range p.IDs {
					ids = append(ids, id)
				}
				if p.Played {
					err = client.MarkRead(ctx, ids, ts, chat, sender, types.ReceiptTypePlayed)
				} else {
					err = client.MarkRead(ctx, ids, ts, chat, sender)
				}
				if err != nil {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "error", "error": err.Error()})
				} else {
					sendIPC("response", map[string]interface{}{"id": cmd.ID, "status": "ok", "resp": true})
				}

			// Group Metadata
			case "getGroupMetadata":
				var p GroupJIDPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						info, err := client.GetGroupInfo(ctx, targetJID)

						if err == nil {
							var participants []map[string]interface{}
							isBotAdmin := false

							botUser := ""
							botLidUser := ""

							if client.Store.ID != nil {
								botUser = client.Store.ID.User
							}

							if client.Store.LID.User != "" {
								botLidUser = client.Store.LID.User
							}

							for _, m := range info.Participants {
								adminType := ""

								if m.IsSuperAdmin {
									adminType = "superadmin"
								} else if m.IsAdmin {
									adminType = "admin"
								}

								pUser := m.JID.User

								isBot :=
									(botUser != "" && pUser == botUser) ||
										(botLidUser != "" && pUser == botLidUser)

								if isBot &&
									(m.IsAdmin || m.IsSuperAdmin) {
									isBotAdmin = true
								}

								participants = append(
									participants,
									map[string]interface{}{
										"id":    m.JID.String(),
										"user":  m.JID.User,
										"admin": adminType,
										"isBot": isBot,
									},
								)
							}

							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
								"resp": map[string]interface{}{
									"id":           info.JID.String(),
									"subject":      info.Name,
									"owner":        info.OwnerJID.String(),
									"isBotAdmin":   isBotAdmin,
									"participants": participants,
								},
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Invite Link
			case "getGroupInviteLink":
				var p GroupJIDPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						link, err := client.GetGroupInviteLink(
							ctx,
							targetJID,
							false,
						)

						if err == nil {
							if !strings.HasPrefix(link, "https://") {
								link =
									"https://chat.whatsapp.com/" +
										link
							}

							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
								"resp":   link,
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Group Participants
			case "updateGroupParticipants":
				var p GroupParticipantsPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Invalid Group JID",
						})
						continue
					}

					var participantJIDs []types.JID

					for _, user := range p.Participants {
						uJID, err := types.ParseJID(user)

						if err == nil {
							participantJIDs =
								append(participantJIDs, uJID)
						}
					}

					var waAction whatsmeow.ParticipantChange

					switch p.Action {

					case "add":
						waAction = whatsmeow.ParticipantChangeAdd

					case "remove":
						waAction = whatsmeow.ParticipantChangeRemove

					case "promote":
						waAction =
							whatsmeow.ParticipantChangePromote

					case "demote":
						waAction =
							whatsmeow.ParticipantChangeDemote

					default:
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Aksi tidak dikenal",
						})
						continue
					}

					res, err := client.UpdateGroupParticipants(
						ctx,
						targetJID,
						participantJIDs,
						waAction,
					)

					if err == nil {
						inviteRequired := false
						var inviteLink string

						if p.Action == "add" {
							for _, item := range res {
								if item.Error == 403 {
									inviteRequired = true
									break
								}
							}

							if inviteRequired {
								link, errLink :=
									client.GetGroupInviteLink(
										ctx,
										targetJID,
										false,
									)

								if errLink == nil {
									if !strings.HasPrefix(
										link,
										"https://",
									) {
										link =
											"https://chat.whatsapp.com/" +
												link
									}

									inviteLink = link
								}
							}
						}

						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "ok",
							"resp": map[string]interface{}{
								"results":        res,
								"inviteRequired": inviteRequired,
								"inviteLink":     inviteLink,
							},
						})
					} else {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  err.Error(),
						})
					}
				}

			// Group Settings
			case "updateGroupSettings":
				var p GroupSettingPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Invalid Group JID",
						})
						continue
					}

					switch p.Setting {

					case "announcement":
						err = client.SetGroupAnnounce(
							ctx,
							targetJID,
							true,
						)

					case "not_announcement":
						err = client.SetGroupAnnounce(
							ctx,
							targetJID,
							false,
						)

					case "locked":
						err = client.SetGroupLocked(
							ctx,
							targetJID,
							true,
						)

					case "unlocked":
						err = client.SetGroupLocked(
							ctx,
							targetJID,
							false,
						)

					default:
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Setelan tidak dikenal",
						})
						continue
					}

					if err == nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "ok",
							"resp":   "success",
						})
					} else {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  err.Error(),
						})
					}
				}

			// Group Subject
			case "setGroupSubject":
				var p struct {
					JID     string `json:"jid"`
					Subject string `json:"subject"`
				}

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						err = client.SetGroupName(
							ctx,
							targetJID,
							p.Subject,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Group Description
			case "setGroupDescription":
				var p struct {
					JID         string `json:"jid"`
					Description string `json:"description"`
				}

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						err = client.SetGroupTopic(
							ctx,
							targetJID,
							"",
							"",
							p.Description,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Revoke Group Invite Link
			case "revokeGroupInviteLink":
				var p GroupJIDPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						link, err := client.GetGroupInviteLink(
							ctx,
							targetJID,
							true,
						)

						if err == nil {
							if !strings.HasPrefix(link, "https://") {
								link =
									"https://chat.whatsapp.com/" +
										link
							}

							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
								"resp":   link,
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Download Media
			case "downloadMedia":
				var p DownloadMediaPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					msgCacheMutex.RLock()
					evtMsg, exists := msgCache[p.MessageID]
					msgCacheMutex.RUnlock()

					if !exists {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Pesan tidak ditemukan di cache",
						})
						continue
					}

					mediaMsg, ext, defaultName :=
						extractMediaMessage(evtMsg.Message)

					if mediaMsg == nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Pesan tidak mengandung media",
						})
						continue
					}

					data, err := client.DownloadAny(
						ctx,
						mediaMsg,
					)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Gagal download media: " + err.Error(),
						})
						continue
					}

					outputDir := p.OutputDir

					if outputDir == "" {
						outputDir = "."
					}

					_ = os.MkdirAll(outputDir, 0755)

					fileName := fmt.Sprintf(
						"%s%s",
						defaultName,
						ext,
					)

					finalPath :=
						filepath.Join(outputDir, fileName)

					counter := 2

					for {
						if _, err := os.Stat(finalPath); os.IsNotExist(err) {
							break
						}

						fileName = fmt.Sprintf(
							"%s%d%s",
							defaultName,
							counter,
							ext,
						)

						finalPath =
							filepath.Join(outputDir, fileName)

						counter++
					}

					err = os.WriteFile(
						finalPath,
						data,
						0644,
					)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Gagal simpan file: " + err.Error(),
						})
						continue
					}

					sendIPC("response", map[string]interface{}{
						"id":     cmd.ID,
						"status": "ok",
						"resp": map[string]interface{}{
							"filePath":  finalPath,
							"fileName":  fileName,
							"mediaType": defaultName,
							"ext":       ext,
						},
					})
				}

			// Send Message
			case "sendMessage":
				var p SendMessagePayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						var msg *waProto.Message

						if p.QuotedID != "" {
							contextInfo := &waProto.ContextInfo{
								StanzaID:    &p.QuotedID,
								Participant: &p.QuotedSender,
							}

							msg = &waProto.Message{
								ExtendedTextMessage: &waProto.ExtendedTextMessage{
									Text:        &p.Text,
									ContextInfo: contextInfo,
								},
							}
						} else {
							msg = &waProto.Message{
								Conversation: &p.Text,
							}
						}

						resp, err := client.SendMessage(
							ctx,
							targetJID,
							msg,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
								"resp":   resp,
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// React Message
			case "reactMessage":
				var p struct {
					JID string `json:"jid"`
					Key struct {
						ID          string `json:"id"`
						Participant string `json:"participant"`
					} `json:"key"`
					Text string `json:"text"`
				}

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						var senderJID types.JID

						if p.Key.Participant != "" {
							senderJID, _ =
								types.ParseJID(
									p.Key.Participant,
								)
						}

						reactMsg := client.BuildReaction(
							targetJID,
							senderJID,
							p.Key.ID,
							p.Text,
						)

						_, err = client.SendMessage(
							ctx,
							targetJID,
							reactMsg,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Delete Message
			case "deleteMessage":
				var p struct {
					JID string `json:"jid"`
					Key struct {
						ID          string `json:"id"`
						Participant string `json:"participant"`
					} `json:"key"`
				}

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						var senderJID types.JID

						if p.Key.Participant != "" {
							senderJID, _ =
								types.ParseJID(
									p.Key.Participant,
								)
						}

						revokeMsg := client.BuildRevoke(
							targetJID,
							senderJID,
							p.Key.ID,
						)

						_, err = client.SendMessage(
							ctx,
							targetJID,
							revokeMsg,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Edit Message
			case "editMessage":
				var p struct {
					JID string `json:"jid"`
					Key struct {
						ID string `json:"id"`
					} `json:"key"`
					Text string `json:"text"`
				}

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err == nil {
						newContent := &waE2E.Message{
							Conversation: proto.String(p.Text),
						}

						editMsg := client.BuildEdit(
							targetJID,
							p.Key.ID,
							newContent,
						)

						_, err = client.SendMessage(
							ctx,
							targetJID,
							editMsg,
						)

						if err == nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "ok",
							})
						} else {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  err.Error(),
							})
						}
					}
				}

			// Send Media
			case "sendMedia":
				var p SendMediaPayload

				if err := json.Unmarshal(cmd.Payload, &p); err == nil {
					targetJID, err := types.ParseJID(p.JID)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Invalid JID",
						})
						continue
					}

					var fileData []byte

					if strings.HasPrefix(p.FilePath, "http://") || strings.HasPrefix(p.FilePath, "https://") {
						resp, err := http.Get(p.FilePath)

						if err != nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  "Gagal mengambil URL: " + err.Error(),
							})
							continue
						}

						if resp.StatusCode < 200 || resp.StatusCode >= 300 {
							resp.Body.Close()

							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  fmt.Sprintf("Gagal mengambil URL: HTTP %d", resp.StatusCode),
							})
							continue
						}

						fileData, err = io.ReadAll(resp.Body)
						resp.Body.Close()

						if err != nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  "Gagal membaca URL: " + err.Error(),
							})
							continue
						}
					} else {
						fileData, err = os.ReadFile(p.FilePath)

						if err != nil {
							sendIPC("response", map[string]interface{}{
								"id":     cmd.ID,
								"status": "error",
								"error":  "File tidak ditemukan: " + err.Error(),
							})
							continue
						}
					}

					var waMediaType whatsmeow.MediaType

					switch p.MediaType {

					case "image", "sticker":
						waMediaType = whatsmeow.MediaImage

					case "video":
						waMediaType = whatsmeow.MediaVideo

					case "audio", "sound", "ptt":
						waMediaType = whatsmeow.MediaAudio

					default:
						waMediaType = whatsmeow.MediaDocument
					}

					uploadResp, err := client.Upload(
						ctx,
						fileData,
						waMediaType,
					)

					if err != nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  "Gagal Upload: " + err.Error(),
						})
						continue
					}

					mimeType :=
						http.DetectContentType(fileData)

					fileLen := uint64(len(fileData))

					var contextInfo *waProto.ContextInfo

					if p.QuotedID != "" {
						contextInfo = &waProto.ContextInfo{
							StanzaID:    &p.QuotedID,
							Participant: &p.QuotedSender,
						}
					}

					var msg *waProto.Message

					switch p.MediaType {

					case "image":
						msg = &waProto.Message{
							ImageMessage: &waProto.ImageMessage{
								URL:           &uploadResp.URL,
								DirectPath:    &uploadResp.DirectPath,
								MediaKey:      uploadResp.MediaKey,
								FileSHA256:    uploadResp.FileSHA256,
								FileEncSHA256: uploadResp.FileEncSHA256,
								FileLength:    &fileLen,
								Mimetype:      &mimeType,
								Caption:       &p.Caption,
								ContextInfo:   contextInfo,
							},
						}

					// Lenwy Was Here: Sticker Support
					case "sticker":
						mimeSticker := "image/webp"

						msg = &waProto.Message{
							StickerMessage: &waProto.StickerMessage{
								URL:           &uploadResp.URL,
								DirectPath:    &uploadResp.DirectPath,
								MediaKey:      uploadResp.MediaKey,
								FileSHA256:    uploadResp.FileSHA256,
								FileEncSHA256: uploadResp.FileEncSHA256,
								FileLength:    &fileLen,
								Mimetype:      &mimeSticker,
								ContextInfo:   contextInfo,
							},
						}

					case "video":
						mimeVideo := "video/mp4"

						msg = &waProto.Message{
							VideoMessage: &waProto.VideoMessage{
								URL:           &uploadResp.URL,
								DirectPath:    &uploadResp.DirectPath,
								MediaKey:      uploadResp.MediaKey,
								FileSHA256:    uploadResp.FileSHA256,
								FileEncSHA256: uploadResp.FileEncSHA256,
								FileLength:    &fileLen,
								Mimetype:      &mimeVideo,
								Caption:       &p.Caption,
								ContextInfo:   contextInfo,
							},
						}

					case "audio", "sound", "ptt":
						isPTT := p.MediaType == "ptt"
						mimeAudio := mimeType

						if isPTT {
							mimeAudio =
								"audio/ogg; codecs=opus"
						}

						msg = &waProto.Message{
							AudioMessage: &waProto.AudioMessage{
								URL:           &uploadResp.URL,
								DirectPath:    &uploadResp.DirectPath,
								MediaKey:      uploadResp.MediaKey,
								FileSHA256:    uploadResp.FileSHA256,
								FileEncSHA256: uploadResp.FileEncSHA256,
								FileLength:    &fileLen,
								Mimetype:      &mimeAudio,
								PTT:           &isPTT,
								ContextInfo:   contextInfo,
							},
						}

					default:
						docName := p.FileName

						if docName == "" {
							docName =
								filepath.Base(p.FilePath)
						}

						msg = &waProto.Message{
							DocumentMessage: &waProto.DocumentMessage{
								URL:           &uploadResp.URL,
								DirectPath:    &uploadResp.DirectPath,
								MediaKey:      uploadResp.MediaKey,
								FileSHA256:    uploadResp.FileSHA256,
								FileEncSHA256: uploadResp.FileEncSHA256,
								FileLength:    &fileLen,
								Mimetype:      &mimeType,
								Title:         &docName,
								FileName:      &docName,
								ContextInfo:   contextInfo,
							},
						}
					}

					resp, err := client.SendMessage(
						ctx,
						targetJID,
						msg,
					)

					if err == nil {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "ok",
							"resp":   resp,
						})
					} else {
						sendIPC("response", map[string]interface{}{
							"id":     cmd.ID,
							"status": "error",
							"error":  err.Error(),
						})
					}
				}
			}
		}
	}()

	sigChan := make(chan os.Signal, 1)

	signal.Notify(
		sigChan,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-sigChan

	fmt.Println(
		"\n[Go Engine] Menerima sinyal shutdown, menutup koneksi...",
	)

	if client.IsConnected() {
		client.Disconnect()
	}

	container.Close()
}
