import { MoviPlayer, AudioRenderer } from 'https://cdn.jsdelivr.net/npm/movi-player@0.4.0/dist/player.js';
import { createSubtitleRenderer, textSubtitleCodecs } from './subtitles.js';

const codecLabels = { ac3: 'AC-3', eac3: 'E-AC-3', dca: 'DTS', dts: 'DTS', subrip: 'SRT', mov_text: 'Text' };

window.createBrowserAVPlay = function (rewriteServerURL) {
  let configuration, current;
  let initialized = false, audioUnlocked = false, audioUnlockPending = false;
  let volume = 0.5, muted = false;
  let displayRect = { top: 0, left: 0, width: 960, height: 540 };

  function callback(group, name, value) {
    configuration?.[group]?.[name]?.(value);
  }

  function buffering(session, active) {
    if (session !== current || session.buffering === active) return;
    session.buffering = active;
    callback('bufferingCallback', active ? 'onbufferingstart' : 'onbufferingcomplete');
  }

  function fail(session, error, failure) {
    if (session !== current || session.failed) return;
    session.failed = true;
    console.error('[White Raven player]', error);
    const problem = error instanceof Error ? error : new Error(String(error));
    if (failure) failure(problem);
    else callback('playCallback', 'onerror', problem);
  }

  // Serialize AVPlay commands. Every file owns its player and queue, so late
  // completions from a stopped file cannot change the next playback session.
  function enqueue(session, operation) {
    const task = session.queue.then(() => {
      if (session !== current || session.failed) return false;
      return operation();
    });
    session.queue = task.catch(() => {});
    return task;
  }

  function captureStreams(session) {
    session.streams = session.engine.getTracks().map(stream => {
      const format = codecLabels[stream.codec] || stream.codec.toUpperCase();
      return {
        ...stream,
        language: stream.language === 'und' ? '' : stream.language || '',
        title: stream.label || '',
        format,
        label: stream.label || '',
      };
    });
    session.duration = Math.max(0, session.engine.getDuration() * 1000);
  }

  function tracks(type) {
    if (!current?.loaded) return [];
    const selected = type === 'audio' ? current.audioId : current.subtitleVisible ? current.subtitleId : -1;
    return current.streams.filter(stream => stream.type === type &&
      (type !== 'subtitle' || textSubtitleCodecs.has(stream.codec.toLowerCase()))).map((stream, index) => ({
      ...stream, index, selected: stream.id === selected,
    }));
  }

  function layout() {
    const box = document.getElementById('playerbox960x540');
    const rect = displayRect;
    const fullscreen = rect.left === 0 && rect.width === 960;
    Object.assign(box.style, {
      position: 'absolute', backgroundColor: '#000',
      left: (fullscreen ? 0 : rect.left) + 'px', top: (fullscreen ? 0 : rect.top) + 'px',
      width: (fullscreen ? 960 : rect.width) + 'px', height: (fullscreen ? 540 : rect.height) + 'px',
    });
    if (current) {
      Object.assign(current.surface.style, {
        left: (fullscreen ? rect.left : 0) + 'px', top: (fullscreen ? rect.top : 0) + 'px',
        width: rect.width + 'px', height: rect.height + 'px',
      });
      current.engine.resizeCanvas(rect.width, rect.height);
    }
  }

  function play(success, failure) {
    const session = current;
    if (!session) return false;
    adapter.status = 'PLAYING';
    if (session.playPending) return true;
    session.playPending = true;
    enqueue(session, async () => {
      if (adapter.status !== 'PLAYING') return false;
      await session.engine.play();
      if (session !== current) return false;
      session.started = true;
      if (adapter.status === 'PAUSED') session.engine.pause();
      buffering(session, false);
      if (success) success();
      return true;
    }).catch(error => fail(session, error, failure)).finally(() => { session.playPending = false; });
    return true;
  }

  function seek(delta) {
    const session = current;
    if (!session?.loaded || !session.started) return false;
    buffering(session, true);
    enqueue(session, async () => {
      const target = Math.max(0, Math.min(session.duration / 1000, session.engine.getCurrentTime() + delta));
      await session.engine.seek(target);
      if (session !== current) return false;
      callback('playCallback', 'oncurrentplaytime', { millisecond: session.engine.getCurrentTime() * 1000 });
      buffering(session, false);
      return true;
    }).catch(error => fail(session, error));
    return true;
  }

  const adapter = {
    status: 'IDLE',
    _getAllInstance: () => initialized ? { browser: adapter } : {},
    getAVPlay: success => success(adapter),
    init(options) {
      configuration = options;
      initialized = true;
      adapter.setDisplayRect(options.displayRect || displayRect);
    },
    open(url) {
      adapter.stop();
      // Movi manages the canvas's own size/rotation styles and sets it to fill
      // its parent. Keep AVPlay's display rectangle on a separate container.
      const surface = document.createElement('div');
      surface.className = 'browser-media';
      const canvas = document.createElement('canvas');
      surface.appendChild(canvas);
      document.getElementById('playerbox960x540').appendChild(surface);
      const engine = new MoviPlayer({
        source: { type: 'url', url: rewriteServerURL(url) },
        canvas, renderer: 'canvas', decoder: 'auto', enablePreviews: false,
      });
      engine.setFitMode('contain');
      const session = current = {
        engine, surface, queue: Promise.resolve(), streams: [], duration: 0,
        loaded: false, started: false, failed: false, buffering: false,
        audioId: -1, subtitleId: -1, subtitleVisible: false, subtitleRevision: 0,
      };
      session.subtitles = createSubtitleRenderer(text => {
        if (session === current) callback('playCallback', 'onsubtitle', text);
      });
      engine.setSubtitleRenderer(session.subtitles);
      adapter.status = 'READY';
      layout();
      engine.setVolume(volume);
      engine.setMuted(muted);
      engine.on('timeUpdate', time => {
        if (session === current) callback('playCallback', 'oncurrentplaytime', { millisecond: time * 1000 });
      });
      engine.on('stateChange', state => {
        if (session !== current) return;
        if (state === 'ended') callback('playCallback', 'onstreamcompleted');
        else if (state === 'buffering') buffering(session, true);
        else if (state === 'playing') buffering(session, false);
      });
      engine.on('error', error => fail(session, error));
      engine.on('tracksChange', () => { if (session === current && session.loaded) captureStreams(session); });
      buffering(session, true);
      enqueue(session, async () => {
        await engine.load();
        if (session !== current) return false;
        session.loaded = true;
        captureStreams(session);
        session.audioId = session.streams.find(stream => stream.type === 'audio')?.id ?? -1;
        // Read the first text track while hidden, so enabling it can display
        // a cue whose packet has already passed. Image tracks need another renderer.
        const subtitle = tracks('subtitle')[0];
        if (subtitle && await engine.selectSubtitleTrack(subtitle.id)) session.subtitleId = subtitle.id;
        else await engine.selectSubtitleTrack(null);
        return true;
      }).catch(error => fail(session, error));
    },
    play,
    pause() {
      if (!current) return false;
      const session = current;
      adapter.status = 'PAUSED';
      enqueue(session, () => { if (session.started) session.engine.pause(); }).catch(error => fail(session, error));
      return true;
    },
    resume() {
      if (!current) return false;
      if (adapter.status !== 'PLAYING') return play();
      return true;
    },
    stop() {
      const session = current;
      current = null;
      adapter.status = 'IDLE';
      callback('playCallback', 'onsubtitle', '');
      if (session) {
        session.engine.destroy(); // Also aborts in-flight media requests.
        session.subtitles.destroy();
        session.surface.remove();
      }
      return true;
    },
    restoreBrowserAudio() {
      if (audioUnlocked || audioUnlockPending) return;
      // Unlock Movi's shared context on Enter, before loading a stream loses the gesture.
      const audio = new AudioRenderer();
      audioUnlockPending = true;
      audio.play().then(() => { audioUnlocked = audio.wasEverActivated(); })
        .catch(console.warn).finally(() => { audio.destroy(); audioUnlockPending = false; });
    },
    setVolume(value, userMuted) {
      volume = Math.max(0, Math.min(1, value));
      muted = !!userMuted;
      current?.engine.setVolume(volume);
      current?.engine.setMuted(muted);
    },
    jumpForward: seek,
    jumpBackward: seconds => seek(-seconds),
    getDuration: () => current?.duration || 0,
    getVideoResolution: () => adapter.videoWidth + '|' + adapter.videoHeight,
    getCurrentBitrate: () => -1,
    getAudioTracks: () => tracks('audio'),
    getSubtitleTracks: () => tracks('subtitle'),
    setAudioStreamID(index) {
      const session = current;
      const track = tracks('audio').find(track => track.index === index);
      if (!session || !track) return false;
      return enqueue(session, async () => {
        if (!session.engine.selectAudioTrack(track.id)) return false;
        session.audioId = track.id;
        // Re-read at the playhead so already-buffered sound from the old track is flushed.
        await session.engine.seek(session.engine.getCurrentTime());
        return session === current;
      }).catch(error => { console.warn('[White Raven audio selection]', error); return false; });
    },
    setSubtitleStreamID(index) {
      const session = current;
      const track = tracks('subtitle').find(track => track.index === index);
      if (!session?.loaded || !track) return false;
      const revision = ++session.subtitleRevision;
      return enqueue(session, async () => {
        if (session.subtitleId !== track.id) {
          if (!await session.engine.selectSubtitleTrack(track.id)) return false;
          session.subtitleId = track.id;
          // Subtitle selection can happen after the demuxer read this moment.
          await session.engine.seek(session.engine.getCurrentTime());
        }
        if (session !== current || revision !== session.subtitleRevision) return false;
        session.subtitleVisible = true;
        session.subtitles.setDelay(0);
        session.subtitles.setVisible(true);
        return true;
      }).catch(error => { console.warn('[White Raven subtitle selection]', error); return false; });
    },
    stopSubtitle() {
      if (current) {
        current.subtitleRevision++;
        current.subtitleVisible = false;
        current.subtitles.setVisible(false);
      }
      callback('playCallback', 'onsubtitle', '');
    },
    // The widget's positive offset advances captions. No full-file subtitle scan is needed.
    setSubtitleSync(milliseconds) { current?.subtitles.setDelay(-milliseconds / 1000); },
    setInitialBufferSize() {}, setPendingBufferSize() {}, setTotalBufferSize() {},
    setDisplayArea: rect => adapter.setDisplayRect(rect),
    setDisplayRect(rect) { displayRect = rect; layout(); },
  };
  Object.defineProperties(adapter, {
    totalNumOfAudio: { get: () => tracks('audio').length },
    videoWidth: { get: () => current?.streams.find(stream => stream.type === 'video')?.width || 0 },
    videoHeight: { get: () => current?.streams.find(stream => stream.type === 'video')?.height || 0 },
  });
  return adapter;
};
