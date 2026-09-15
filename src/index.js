import { WhatsMeowBridge } from "./bridge.js";
import { chatLog } from "./chatlog.js";
import { extractNumber, isLid, isGroup, jidNormalizedUser } from "./utils.js";

export function makeWASocket(options = {}) {
  const bridge = new WhatsMeowBridge(options);

  return {
    ev: bridge,
    start: () => bridge.start(),
    stop: () => bridge.stop(),
    end: () => bridge.stop(),
    requestPairingCode: (phone) => bridge.requestPairingCode(phone),
    profilePictureUrl: (jid, type = "image") => bridge.profilePictureUrl(jid, type),
    getUserInfo: (jids) => bridge.getUserInfo(jids),
    isOnWhatsApp: (phones) => bridge.isOnWhatsApp(phones),
    getJoinedGroups: () => bridge.getJoinedGroups(),
    getGroupInfoFromLink: (code) => bridge.getGroupInfoFromLink(code),
    joinGroupWithLink: (code) => bridge.joinGroupWithLink(code),
    leaveGroup: (jid) => bridge.leaveGroup(jid),
    getBusinessProfile: (jid) => bridge.getBusinessProfile(jid),
    sendPresence: (state) => bridge.sendPresence(state),
    sendChatPresence: (jid, state, media) =>
      bridge.sendChatPresence(jid, state, media),
    subscribePresence: (jid) => bridge.subscribePresence(jid),
    markRead: (ids, timestamp, chat, sender, played) =>
      bridge.markRead(ids, timestamp, chat, sender, played),
    sendMessage: (jid, content, options) =>
      bridge.sendMessage(jid, content, options),
    downloadMedia: (target, outputDir) =>
      bridge.downloadMedia(target, outputDir),
    groupMetadata: (jid) => bridge.groupMetadata(jid),
    groupParticipantsUpdate: (jid, participants, action) =>
      bridge.groupParticipantsUpdate(jid, participants, action),
    groupSettingUpdate: (jid, setting) =>
      bridge.groupSettingUpdate(jid, setting),
    groupInviteCode: (jid) => bridge.groupInviteCode(jid),
    groupRevokeInviteCode: (jid) => bridge.groupRevokeInviteCode(jid),
    groupUpdateSubject: (jid, subject) =>
      bridge.groupUpdateSubject(jid, subject),
    groupUpdateDescription: (jid, description) =>
      bridge.groupUpdateDescription(jid, description),
    react: (jid, key, text) => bridge.react(jid, key, text),
    deleteMessage: (jid, key) => bridge.deleteMessage(jid, key),
    editMessage: (jid, key, text) => bridge.editMessage(jid, key, text),
  };
}

export { chatLog, extractNumber, isLid, isGroup, jidNormalizedUser };
