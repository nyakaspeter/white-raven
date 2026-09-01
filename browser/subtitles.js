// Movi supplies demuxed packets and the media clock. Only text presentation is
// ours: feed the same AVPlay subtitle callback used by the Samsung widget.
export const textSubtitleCodecs = new Set(['subrip', 'srt', 'ass', 'ssa', 'webvtt', 'mov_text', 'text']);

export function createSubtitleRenderer(onText) {
  const decoder = new TextDecoder();
  let codec, streamId, cues = [], visible = false, delay = 0, lastText = null;

  function emit(text) {
    if (text === lastText) return;
    lastText = text;
    onText(text);
  }

  return {
    configure(track) {
      codec = track.codec.toLowerCase();
      streamId = track.id;
      cues = [];
      this.clear();
    },
    pushPacket(packet) {
      if (packet.streamIndex !== streamId || !textSubtitleCodecs.has(codec)) return;
      let bytes = packet.data;
      if (codec === 'mov_text') {
        // tx3g packets begin with a big-endian text length; ignore style boxes.
        if (bytes.length < 2) return;
        const length = (bytes[0] << 8) | bytes[1];
        bytes = bytes.subarray(2, Math.min(bytes.length, length + 2));
      }
      const utf16 = bytes[0] === 0xfe && bytes[1] === 0xff ? 'utf-16be'
        : bytes[0] === 0xff && bytes[1] === 0xfe ? 'utf-16le' : null;
      let text = (utf16 ? new TextDecoder(utf16) : decoder).decode(bytes).replace(/\u0000/g, '');
      if (codec === 'ass' || codec === 'ssa') {
        // Matroska ASS packets have eight comma-delimited fields before Text.
        // Dialogue-form SSA has nine. Preserve commas inside the dialogue.
        const fields = /^Dialogue:\s*/i.test(text) ? 9 : 8;
        text = text.replace(/^Dialogue:\s*/i, '');
        let start = 0;
        for (let field = 0; field < fields; field++) {
          const comma = text.indexOf(',', start);
          if (comma < 0) return;
          start = comma + 1;
        }
        text = text.slice(start).replace(/\{[^}]*\}/g, '')
          .replace(/\\[Nn]/g, '\n').replace(/\\h/g, '\u00a0');
      }
      if (!text.trim() || !Number.isFinite(packet.timestamp)) return;
      const start = packet.timestamp;
      const end = start + (packet.duration > 0 ? packet.duration : 5);
      if (!cues.some(cue => cue.start === start && cue.end === end && cue.text === text)) {
        cues.push({ start, end, text });
      }
    },
    render(mediaTime) {
      const time = mediaTime - delay;
      // Retain a small history for sync adjustments; seek clears and refills it.
      cues = cues.filter(cue => cue.end >= time - 60);
      emit(visible ? cues.filter(cue => cue.start <= time && time < cue.end)
        .map(cue => cue.text).join('\n') : '');
    },
    setVisible(value) {
      visible = value;
      lastText = null;
      if (!visible) emit('');
    },
    setDelay(seconds) { delay = seconds; },
    // A seek can land after the start of a long cue. Keep already-read cues
    // for this track; configure() resets them when the selected track changes.
    clear() { lastText = null; emit(''); },
    destroy() { cues = []; this.clear(); },
  };
}
