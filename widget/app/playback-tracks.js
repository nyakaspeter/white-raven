/* Native track access for the browser adapter and Samsung's legacy AVPlay.
 * Orsay stream types: 1 = audio, 4 = embedded subtitle (5 = external subtitle).
 * Keep this file ES5-compatible for E/F/H-series televisions.
 */
function PlaybackTracks(player, onSubtitle) {
    this.player = player;
    this.onSubtitle = onSubtitle;
    this.subtitleIndex = -1;
    this.subtitleRequest = 0;
    this.subtitleActive = false;
    this.subtitleText = '';
    this.pendingSubtitle = null;
    this.nativeSubtitleIndex = -1;
    this.nativeSubtitleOffset = 0;
    this.nativeSubtitleInitialized = false;
    this.nativeSubtitleTimer = null;
    this.subtitleCueReceived = false;
    // Legacy AVPlay starts on audio stream zero; remember subsequent switches.
    this.audioIndex = window.WHITE_RAVEN_BROWSER ? -1 : 0;
    this.ready = !!window.WHITE_RAVEN_BROWSER;
    var self = this;
    if (!window.WHITE_RAVEN_BROWSER && typeof player.onEvent === 'function') {
        this.originalEvent = player.onEvent;
        this.eventHandler = function(type, data) {
            // Route native text directly to our renderer instead of AVPlay's
            // external-subtitle callback path.
            if (Number(type) === 19) {
                self.renderSubtitle(data);
                return;
            }
            if (Number(type) === 14) self.ready = true;
            if (Number(type) === 11) self.ready = false;
            if (Number(type) === 11) {
                self.subtitleText = '';
                if (self.subtitleActive) self.onSubtitle('');
            }
            // Preserve AVPlay's buffering, playback and error callbacks.
            var result = self.originalEvent.apply(this, arguments);
            if (Number(type) === 9 && !self.nativeSubtitleInitialized) {
                self.nativeSubtitleInitialized = true;
                // STREAM_INFO_READY can recur after seeking. Initialize once,
                // after the native callback returns and before track selection.
                self.nativeSubtitleTimer = setTimeout(function() {
                    self.nativeSubtitleTimer = null;
                    if (player.onEvent !== self.eventHandler || self.list(4).length === 0 ||
                            typeof player.startSubtitle !== 'function') return;
                    try {
                        // Lampa's Orsay startup workaround enables native subtitle
                        // delivery. The invalid external ID leaves embedded track
                        // selection to setStreamID(4, index); keep the offset zero.
                        // https://github.com/yumata/lampa-source/blob/d2c35548c28ecf56b7491ad09d969c2a04a948b8/src/interaction/player/video/orsay.js#L394
                        var started = player.startSubtitle({path: '/dtv/temp/', streamID: 999,
                            sync: 0, callback: function() {}});
                        self.log('Native subtitle startup', started);
                    } catch (error) { self.log('Native subtitle startup error', error); }
                }, 0);
            }
            return result;
        };
        player.onEvent = this.eventHandler;
    }
}

PlaybackTracks.prototype.subtitlesReady = function() {
    return this.ready && this.nativeSubtitleTimer === null;
};

PlaybackTracks.prototype.setSubtitleSync = function(milliseconds) {
    if (typeof this.player.setSubtitleSync === 'function') {
        this.player.setSubtitleSync(milliseconds);
        this.nativeSubtitleOffset = Number(milliseconds);
    }
};

PlaybackTracks.prototype.renderSubtitle = function(text) {
    text = text == null ? '' : String(text);
    if (text && !this.subtitleCueReceived) {
        this.subtitleCueReceived = true;
        this.log('First subtitle text received', text.length + ' characters');
    }
    // A native switch (or Movi enabling its renderer) can deliver a cue before
    // selection returns. Keep it until the scene finishes changing renderers.
    if (this.pendingSubtitle) {
        this.pendingSubtitle.text = text;
        return;
    }
    this.subtitleText = text;
    if (this.subtitleActive) this.onSubtitle(text);
};

PlaybackTracks.prototype.refreshSubtitle = function() {
    if (this.subtitleActive) this.onSubtitle(this.subtitleText);
};

PlaybackTracks.prototype.resetNativeSubtitleSync = function() {
    if (this.nativeSubtitleOffset !== 0 && typeof this.player.setSubtitleSync === 'function') {
        // Only reset an offset the user actually changed.
        try {
            this.player.setSubtitleSync(0);
            this.nativeSubtitleOffset = 0;
        } catch (error) { this.log('Subtitle sync reset', error); }
    }
};

PlaybackTracks.prototype.list = function(type) {
    var player = this.player;
    var getter = type === 1 ? 'getAudioTracks' : 'getSubtitleTracks';
    if (typeof player[getter] === 'function') return player[getter]();
    // Only advertise embedded subtitles when we can receive their text events.
    if (type === 4 && !this.eventHandler) return [];
    var count = 0;
    try {
        if (typeof player.getTotalNumOfStreamID === 'function') count = player.getTotalNumOfStreamID(type);
        else if (type === 1) count = player.totalNumOfAudio;
    } catch (error) { return []; }
    count = Math.max(0, Math.min(100, Number(count) || 0));
    var tracks = [];
    for (var i = 0; i < count; i++) {
        var language = '';
        var label = '';
        var format = '';
        try {
            if (typeof player.getStreamLanguageInfo === 'function') {
                language = PlaybackTrackLanguage(player.getStreamLanguageInfo(type, i));
            }
        } catch (error) { /* Still try the other metadata sources. */ }
        try {
            if (typeof player.getStreamExtraData === 'function') {
                var extra = player.getStreamExtraData(type, i);
                if (typeof extra === 'string') extra = JSON.parse(extra || '{}');
                extra = extra || {};
                language = language || PlaybackTrackLanguage(extra.language || extra.lang);
                label = extra.title || '';
                format = extra.codec || extra.fourCC || '';
            }
        } catch (error) { /* Optional metadata varies by firmware. */ }
        tracks.push({index: i, language: language, title: label, label: label, format: format,
            selected: type === 1 && this.audioIndex === i});
    }
    return tracks;
};

PlaybackTracks.prototype.selectAudio = function(index) {
    var self = this;
    var tracks = this.list(1);
    var track = null;
    for (var i = 0; i < tracks.length; i++) if (tracks[i].index === index) track = tracks[i];
    if (!track) return false;
    // Some firmware rejects switching to the stream that is already playing.
    if (track.selected) return true;
    function selected(result) {
        if (result !== true && result !== 1) return false;
        self.audioIndex = index;
        return true;
    }
    try {
        var result = !window.WHITE_RAVEN_BROWSER && typeof this.player.setStreamID === 'function' ?
            this.player.setStreamID(1, index) : this.player.setAudioStreamID(index);
        if (result && typeof result.then === 'function') return result.then(selected, function() { return false; });
        return selected(result);
    } catch (error) { return false; }
};

PlaybackTracks.prototype.selectSubtitle = function(index) {
    var self = this;
    var request = ++this.subtitleRequest;
    this.pendingSubtitle = null;
    function selected(result) {
        if (request !== self.subtitleRequest) return false;
        var pending = self.pendingSubtitle;
        self.pendingSubtitle = null;
        if (result !== true && result !== 1) return false;
        self.subtitleIndex = index;
        if (!window.WHITE_RAVEN_BROWSER) self.nativeSubtitleIndex = index;
        self.subtitleActive = true;
        self.subtitleText = pending ? pending.text : self.subtitleText;
        return true;
    }
    var tracks = this.list(4);
    var found = false;
    for (var i = 0; i < tracks.length; i++) if (tracks[i].index === index) found = true;
    if (!found) return false;
    // Hiding native subtitles only hides our text layer, so showing that same
    // track again does not require a second SetStreamID call either.
    if ((!window.WHITE_RAVEN_BROWSER && this.nativeSubtitleIndex === index) ||
            (window.WHITE_RAVEN_BROWSER && this.subtitleIndex === index && this.subtitleActive)) {
        if (!window.WHITE_RAVEN_BROWSER) this.resetNativeSubtitleSync();
        this.subtitleIndex = index;
        this.subtitleActive = true;
        return true;
    }
    this.pendingSubtitle = {text: ''};
    this.subtitleCueReceived = false;
    try {
        var result;
        if (window.WHITE_RAVEN_BROWSER) result = this.player.setSubtitleStreamID(index);
        else if (typeof this.player.setStreamID === 'function') {
            // Select only the requested embedded stream.
            result = this.player.setStreamID(4, index);
            if (result === true || result === 1) this.resetNativeSubtitleSync();
        } else return selected(false);
        this.log('Subtitle stream ' + index + ' selection', result);
        if (result && typeof result.then === 'function') return result.then(selected, function() { return selected(false); });
        return selected(result);
    } catch (error) {
        this.log('Subtitle stream ' + index + ' selection error', error);
        return selected(false);
    }
};

PlaybackTracks.prototype.log = function(message, result) {
    if (typeof islogenabled !== 'undefined' && islogenabled === 'true') {
        alert('[PlaybackTracks] ' + message + ': ' + (result && result.message ? result.message : String(result)));
    }
};

PlaybackTracks.prototype.hideSubtitle = function() {
    this.subtitleRequest++;
    this.pendingSubtitle = null;
    this.subtitleActive = false;
    if (window.WHITE_RAVEN_BROWSER) this.player.stopSubtitle();
    this.onSubtitle('');
};

PlaybackTracks.prototype.dispose = function() {
    clearTimeout(this.nativeSubtitleTimer);
    this.nativeSubtitleTimer = null;
    this.hideSubtitle();
    if (this.eventHandler && this.player.onEvent === this.eventHandler) this.player.onEvent = this.originalEvent;
};

function PlaybackTrackLanguage(value) {
    var code = String(value == null ? '' : value).replace(/\u0000/g, '').replace(/^\s+|\s+$/g, '').toLowerCase();
    if (/^-?\d+$/.test(code)) {
        var packed = Number(code);
        if (packed <= 0 || packed > 16777215) return '';
        code = String.fromCharCode((packed >> 16) & 255, (packed >> 8) & 255, packed & 255).toLowerCase();
        if (!/^[a-z]{3}$/.test(code)) return '';
    }
    return code === 'und' || code === 'unknown' ? '' : code;
}

function PlaybackLanguageLabel(language, fallback) {
    var code = PlaybackTrackLanguage(language).split(/[-_]/)[0];
    var iso3 = {
        ara: 'ar', bul: 'bg', hrv: 'hr', ces: 'cs', cze: 'cs', dan: 'da', nld: 'nl', dut: 'nl',
        eng: 'en', est: 'et', fin: 'fi', fra: 'fr', fre: 'fr', deu: 'de', ger: 'de', ell: 'el',
        gre: 'el', heb: 'he', hun: 'hu', ind: 'id', ita: 'it', kor: 'ko', lav: 'lv', lit: 'lt',
        nor: 'no', fas: 'fa', per: 'fa', pol: 'pl', por: 'pt', ron: 'ro', rum: 'ro', rus: 'ru',
        srp: 'sr', slk: 'sk', slo: 'sk', spa: 'es', swa: 'sw', swe: 'sv', tha: 'th', tur: 'tr',
        urd: 'ur', vie: 'vi', jpn: 'ja', chi: 'zh', zho: 'zh', may: 'ms', msa: 'ms',
        slv: 'sl', ukr: 'uk'
    };
    if (iso3[code]) code = iso3[code];
    var position = languageListText['shortcode'].indexOf(code);
    if (position > 0 && languageListText[lang]) return languageListText[lang][position].toUpperCase();
    return code && code !== 'und' ? code.toUpperCase() : String(fallback || '').toUpperCase();
}

function PlaybackAudioTrackLabel(track, fallback) {
    var parts = [PlaybackLanguageLabel(track.language, fallback)];
    if (track.format) parts.push(String(track.format).toUpperCase());
    var title = track.title || track.label;
    if (title) parts.push(String(title));
    return parts.join(' · ');
}

function PlaybackSubtitleTrackLabel(track) {
    var language = PlaybackLanguageLabel(track.language, 'UND');
    var parts = [language];
    var title = track.title || track.label;
    if (title) parts.push(String(title));
    return parts.join(' · ');
}
