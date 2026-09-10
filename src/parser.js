// Lenwy Was Here: Lenwy Whatsmeow Parser

export function parseToBaileys(raw) {
  const isGroup = raw.chat.endsWith("@g.us");

  const m = {
    messages: [
      {
        key: {
          remoteJid: raw.chat,
          fromMe: raw.isFromMe,
          id: raw.id,
          participant: isGroup ? raw.senderJid : undefined,
        },
        messageTimestamp: raw.timestamp,
        pushName: raw.pushName || "",
        message: {
          conversation: raw.body,
        },
      },
    ],
    type: "notify",
  };

  const meta = {
    body: raw.body || "",
    mediaType: raw.type.toLowerCase(),
    sender: raw.senderJid,
    pushname: raw.pushName || "",
    isGroup,
  };

  return { m, meta };
}
