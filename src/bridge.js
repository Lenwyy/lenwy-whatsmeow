// Lenwy Was Here: Lenwy Whatsmeow Bridge
// Disclaimer: This code is provided as-is and may not be suitable for production use. Use at your own risk.
// Under the MIT License (MIT). Copyright (c) 2024 Lenwy. All rights reserved.

// Thanks to the following libraries:
// - github.com/mattn/go-sqlite3
// - go.mau.fi/whatsmeow
// - github.com/kingard888

import { spawn } from "child_process";
import readline from "readline";
import { EventEmitter } from "events";
import path from "path";
import fs from "fs";
import os from "os";
import { fileURLToPath } from "url";
import { parseToBaileys } from "./parser.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export class WhatsMeowBridge extends EventEmitter {
  constructor(options = {}) {
    super();

    this.sessionName = options.sessionName || "lenwy";
    this.goProcess = null;
    this.pendingRequests = new Map();
    this.messageCache = new Map();
    this.messageCacheLimit = 5000;

    const cleanExit = async () => {
      await this.stop();
      process.exit(0);
    };

    process.once("SIGINT", cleanExit);
    process.once("SIGTERM", cleanExit);
  }

  start() {
    if (this.goProcess && !this.goProcess.killed) {
      return;
    }

    const goPath = path.resolve(__dirname, "../go-engine");
    const platform = os.platform();
    const binaryName = platform === "win32" ? "main_win.exe" : "main_linux";
    const binaryPath = path.join(goPath, binaryName);

    if (platform !== "win32" && fs.existsSync(binaryPath)) {
      try {
        fs.chmodSync(binaryPath, "755");
      } catch (e) {}
    }

    if (!fs.existsSync(binaryPath)) {
      throw new Error(
        `[BRIDGE] Binary engine tidak ditemukan: ${binaryPath}`
      );
    }

    this.goProcess = spawn(binaryPath, [this.sessionName], {
      cwd: goPath,
    });

    const rl = readline.createInterface({
      input: this.goProcess.stdout,
      terminal: false,
    });

    rl.on("line", (line) => {
      try {
        const parsed = JSON.parse(line);
        this._handleIPCEvent(parsed);
      } catch (e) {}
    });

    this.goProcess.stderr.on("data", (data) => {
      console.error("[GO STDERR]", data.toString());
    });

    this.goProcess.on("error", (error) => {
      console.error("[GO PROCESS ERROR]", error);

      for (const { reject } of this.pendingRequests.values()) {
        reject(error);
      }

      this.pendingRequests.clear();
    });

    this.goProcess.on("exit", (code, signal) => {
      console.log(`[GO PROCESS EXIT] code=${code} signal=${signal || "none"}`);

      const error = new Error(
        `Go process berhenti (code=${code}, signal=${signal || "none"})`,
      );

      for (const { reject } of this.pendingRequests.values()) {
        reject(error);
      }

      this.pendingRequests.clear();

      this.goProcess = null;
    });
  }

  async stop() {
    if (!this.goProcess) return;

    const processRef = this.goProcess;

    try {
      const id = Math.random().toString(36).slice(2);
      const command = JSON.stringify({
        action: "shutdown",
        id,
      });

      await Promise.race([
        new Promise((resolve) => {
          this.pendingRequests.set(id, {
            resolve,
            reject: resolve,
          });

          processRef.stdin.write(command + "\n", () => {
            resolve();
          });
        }),

        new Promise((resolve) => {
          setTimeout(resolve, 2000);
        }),
      ]);
    } catch (e) {
    } finally {
      const shutdownId = [...this.pendingRequests.entries()].find(
        ([id]) => id,
      )?.[0];

      if (shutdownId) {
        this.pendingRequests.delete(shutdownId);
      }

      if (processRef && !processRef.killed) {
        processRef.kill("SIGKILL");
      }

      this.goProcess = null;
    }
  }

  _handleIPCEvent(data) {
    if (data.event === "response") {
      const promise = this.pendingRequests.get(data.data.id);

      if (promise) {
        if (data.data.status === "ok") {
          promise.resolve(data.data.resp);
        } else {
          promise.reject(
            new Error(data.data.error || "Unknown Go engine error"),
          );
        }

        this.pendingRequests.delete(data.data.id);
      }

      return;
    }

    if (data.event === "messages.upsert") {
      const raw = data.data || {};
      const { m, meta } = parseToBaileys(raw);

      if (raw.id) {
        this.messageCache.set(raw.id, {
          id: raw.id,
          chat: raw.chat || "",
          senderJid: raw.senderJid || "",
          pushName: raw.pushName || "",
          body: raw.body || "",
          type: raw.type || "Chat",
          timestamp: raw.timestamp || 0,
        });

        if (this.messageCache.size > this.messageCacheLimit) {
          const oldest = this.messageCache.keys().next().value;
          if (oldest) this.messageCache.delete(oldest);
        }
      }

      if (m && raw) {
        m.quotedId = raw.quotedId || "";
        m.quotedSender = raw.quotedSender || "";
        m.quotedType = raw.quotedType || "";
        m.quotedText = raw.quotedText || "";

        if (!m.quotedText && raw.quotedId) {
          const quoted = this.messageCache.get(raw.quotedId);

          if (quoted) {
            m.quotedText = quoted.body || "";
            m.quotedType = quoted.type || "Chat";

            if (!m.quotedSender) {
              m.quotedSender = quoted.senderJid || "";
            }
          }
        }
      }

      this.emit("messages.upsert", {
        m,
        meta,
        raw,
      });

      return;
    }

    this.emit(data.event, data.data);
  }

  async requestPairingCode(phoneNumber) {
    if (!this.goProcess) {
      throw new Error("Go process belum berjalan.");
    }

    const command = JSON.stringify({
      action: "requestPairingCode",
      id: "pairing",
      payload: {
        phone: phoneNumber,
      },
    });

    this.goProcess.stdin.write(command + "\n");
  }

  async getUserInfo(jids) {
    const id = Math.random().toString(36).slice(2);
    const list = Array.isArray(jids) ? jids : [jids];

    return this._sendRequest(
      id,
      JSON.stringify({
        action: "getUserInfo",
        id,
        payload: { jids: list },
      }),
    );
  }

  async isOnWhatsApp(phones) {
    const id = Math.random().toString(36).slice(2);
    const list = Array.isArray(phones) ? phones : [phones];

    return this._sendRequest(
      id,
      JSON.stringify({
        action: "isOnWhatsApp",
        id,
        payload: { phones: list },
      }),
    );
  }

  async getJoinedGroups() {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "getJoinedGroups", id, payload: {} }),
    );
  }

  async getGroupInfoFromLink(code) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "getGroupInfoFromLink",
        id,
        payload: { code },
      }),
    );
  }

  async joinGroupWithLink(code) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "joinGroupWithLink",
        id,
        payload: { code },
      }),
    );
  }

  async leaveGroup(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "leaveGroup",
        id,
        payload: { jid },
      }),
    );
  }

  async newsletterMetadata(type, key) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterMetadata",
        id,
        payload: { type, key },
      }),
    );
  }

  async newsletterFollow(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "newsletterFollow", id, payload: { jid } }),
    );
  }

  async newsletterUnfollow(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "newsletterUnfollow", id, payload: { jid } }),
    );
  }

  async newsletterMute(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "newsletterMute", id, payload: { jid } }),
    );
  }

  async newsletterUnmute(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "newsletterUnmute", id, payload: { jid } }),
    );
  }

  async newsletterCreate(name, description = "") {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterCreate",
        id,
        payload: { name, description },
      }),
    );
  }

  async newsletterReactMessage(jid, serverId, emoji) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterReactMessage",
        id,
        payload: { jid, serverId, reaction: emoji },
      }),
    );
  }

  async newsletterSubscribed() {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({ action: "newsletterSubscribed", id, payload: {} }),
    );
  }

  async newsletterFetchMessages(jid, count = 0, before = 0) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterFetchMessages",
        id,
        payload: { jid, count, before },
      }),
    );
  }

  async newsletterMarkViewed(jid, serverIds) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterMarkViewed",
        id,
        payload: {
          jid,
          serverIds: Array.isArray(serverIds) ? serverIds : [serverIds],
        },
      }),
    );
  }

  async newsletterSubscribeLiveUpdates(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "newsletterSubscribeLiveUpdates",
        id,
        payload: { jid },
      }),
    );
  }

  async getBusinessProfile(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "getBusinessProfile",
        id,
        payload: { jid },
      }),
    );
  }

  async sendPresence(state = "available") {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "sendPresence",
        id,
        payload: { state },
      }),
    );
  }

  async sendChatPresence(jid, state = "composing", media = "text") {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "sendChatPresence",
        id,
        payload: { jid, state, media },
      }),
    );
  }

  async subscribePresence(jid) {
    const id = Math.random().toString(36).slice(2);
    return this._sendRequest(
      id,
      JSON.stringify({
        action: "subscribePresence",
        id,
        payload: { jid },
      }),
    );
  }

  async markRead(ids, timestamp = 0, chat = "", sender = "", played = false) {
    const id = Math.random().toString(36).slice(2);

    if (ids && typeof ids === "object" && !Array.isArray(ids)) {
      const opts = ids;
      ids = opts.ids || [];
      timestamp = opts.timestamp || 0;
      chat = opts.chat || "";
      sender = opts.sender || "";
      played = Boolean(opts.played);
    }

    return this._sendRequest(
      id,
      JSON.stringify({
        action: "markRead",
        id,
        payload: {
          ids: Array.isArray(ids) ? ids : [ids],
          timestamp,
          chat,
          sender,
          played: Boolean(played),
        },
      }),
    );
  }

  async groupMetadata(jid) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "getGroupMetadata",
      id,
      payload: {
        jid,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupParticipantsUpdate(jid, participants, action) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "updateGroupParticipants",
      id,
      payload: {
        jid,
        participants,
        action,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupSettingUpdate(jid, setting) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "updateGroupSettings",
      id,
      payload: {
        jid,
        setting,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupInviteCode(jid) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "getGroupInviteLink",
      id,
      payload: {
        jid,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupUpdateSubject(jid, subject) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "setGroupSubject",
      id,
      payload: {
        jid,
        subject,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupUpdateDescription(jid, description) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "setGroupDescription",
      id,
      payload: {
        jid,
        description,
      },
    });

    return this._sendRequest(id, command);
  }

  async groupRevokeInviteCode(jid) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "revokeGroupInviteLink",
      id,
      payload: {
        jid,
      },
    });

    return this._sendRequest(id, command);
  }

  async react(jid, key, text) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "reactMessage",
      id,
      payload: {
        jid,
        key,
        text,
      },
    });

    return this._sendRequest(id, command);
  }

  async deleteMessage(jid, key) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "deleteMessage",
      id,
      payload: {
        jid,
        key,
      },
    });

    return this._sendRequest(id, command);
  }

  async editMessage(jid, key, text) {
    const id = Math.random().toString(36).slice(2);

    const command = JSON.stringify({
      action: "editMessage",
      id,
      payload: {
        jid,
        key,
        text,
      },
    });

    return this._sendRequest(id, command);
  }

  async downloadMedia(target, outputDir = ".") {
    const id = Math.random().toString(36).slice(2);

    let messageId = "";

    if (typeof target === "string") {
      messageId = target;
    } else if (target?.raw?.id) {
      messageId = target.raw.id;
    } else if (target?.m?.messages?.[0]?.key?.id) {
      messageId = target.m.messages[0].key.id;
    } else if (target?.messages?.[0]?.key?.id) {
      messageId = target.messages[0].key.id;
    } else if (target?.key?.id) {
      messageId = target.key.id;
    } else if (target?.id) {
      messageId = target.id;
    }

    if (!messageId) {
      throw new Error("Message ID tidak ditemukan untuk mengunduh media.");
    }

    const command = JSON.stringify({
      action: "downloadMedia",
      id,
      payload: {
        messageId,
        outputDir,
      },
    });

    return this._sendRequest(id, command);
  }

  async profilePictureUrl(jid, type = "image") {
    const id = Math.random().toString(36).slice(2);

    const payload = {
      jid,
      type: typeof type === "string" ? type : "image",
    };

    const info = await this._sendRequest(
      id,
      JSON.stringify({
        action: "getProfilePicture",
        id,
        payload,
      }),
    );

    if (!info) return undefined;

    return info.url ?? info.URL ?? undefined;
  }

  async sendMessage(jid, content, options = {}) {
    const id = Math.random().toString(36).slice(2);

    let quotedId = "";
    let quotedSender = "";

    if (options.quoted && options.quoted.key) {
      quotedId = options.quoted.key.id || "";
      quotedSender =
        options.quoted.key.participant || options.quoted.key.remoteJid || "";
    }

    if (content.sticker) {
      const filePath =
        typeof content.sticker === "string"
          ? content.sticker
          : content.sticker.url;

      return this._sendMediaIPC(
        id,
        jid,
        "sticker",
        filePath,
        "",
        "",
        quotedId,
        quotedSender,
      );
    }

    if (content.audio || content.ptt) {
      const isPTT = Boolean(content.ptt);
      const mediaType = isPTT ? "ptt" : "audio";
      const mediaObj = content.ptt || content.audio;

      const filePath = typeof mediaObj === "string" ? mediaObj : mediaObj.url;

      return this._sendMediaIPC(
        id,
        jid,
        mediaType,
        filePath,
        "",
        "",
        quotedId,
        quotedSender,
      );
    }

    if (content.video) {
      const filePath =
        typeof content.video === "string" ? content.video : content.video.url;

      return this._sendMediaIPC(
        id,
        jid,
        "video",
        filePath,
        content.caption || "",
        "",
        quotedId,
        quotedSender,
      );
    }

    if (content.image) {
      const filePath =
        typeof content.image === "string" ? content.image : content.image.url;

      return this._sendMediaIPC(
        id,
        jid,
        "image",
        filePath,
        content.caption || "",
        "",
        quotedId,
        quotedSender,
      );
    }

    if (content.document) {
      const filePath =
        typeof content.document === "string"
          ? content.document
          : content.document.url;

      const fileName = content.fileName || path.basename(filePath);

      return this._sendMediaIPC(
        id,
        jid,
        "document",
        filePath,
        "",
        fileName,
        quotedId,
        quotedSender,
      );
    }

    const text = typeof content === "string" ? content : content.text || "";

    const command = JSON.stringify({
      action: "sendMessage",
      id,
      payload: {
        jid,
        text,
        quotedId,
        quotedSender,
      },
    });

    return this._sendRequest(id, command);
  }

  _sendMediaIPC(
    id,
    jid,
    mediaType,
    filePath,
    caption,
    fileName,
    quotedId,
    quotedSender,
  ) {
    const command = JSON.stringify({
      action: "sendMedia",
      id,
      payload: {
        jid,
        mediaType,
        filePath,
        caption,
        fileName,
        quotedId,
        quotedSender,
      },
    });

    return this._sendRequest(id, command);
  }

  _sendRequest(id, command) {
    return new Promise((resolve, reject) => {
      if (
        !this.goProcess ||
        this.goProcess.killed ||
        !this.goProcess.stdin ||
        this.goProcess.stdin.destroyed
      ) {
        reject(new Error("Go process tidak berjalan."));
        return;
      }

      this.pendingRequests.set(id, {
        resolve,
        reject,
      });

      try {
        this.goProcess.stdin.write(command + "\n");
      } catch (error) {
        this.pendingRequests.delete(id);
        reject(error);
      }
    });
  }
}