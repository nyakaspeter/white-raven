(function () {
  "use strict";

  var storagePrefix = "white-raven.browser.";
  var statusElement = document.getElementById("browser-status");
  var currentKeyEvent = null;
  var scenes = {};
  var focusedScene = "";
  var browserRun = Date.now().toString(36);

  function fitViewport() {
    document.getElementById("tv-screen").style.setProperty("--tv-scale",
      Math.min(window.innerWidth / 960, window.innerHeight / 540));
  }
  window.addEventListener("resize", fitViewport);
  fitViewport();

  function devResource(source) {
    return source + (source.indexOf("?") === -1 ? "?" : "&") + "browser-run=" + browserRun;
  }

  function setStatus(message, state) {
    statusElement.textContent = message;
    statusElement.dataset.state = state || "";
  }

  window.WHITE_RAVEN_BROWSER = true;
  window.WHITE_RAVEN_SERVER_URL = window.WHITE_RAVEN_SERVER_URL || "http://127.0.0.1:9000";

  function rewriteServerURL(value) {
    if (typeof value !== "string") {
      return value;
    }
    return value.replace(/^https?:\/\/[^/]+(?=\/(?:api\/v0|file|subtitle)(?:\/|$))/, window.WHITE_RAVEN_SERVER_URL);
  }

  var nativeOpen = XMLHttpRequest.prototype.open;
  XMLHttpRequest.prototype.open = function (method, url) {
    var args = Array.prototype.slice.call(arguments);
    args[1] = rewriteServerURL(url);
    return nativeOpen.apply(this, args);
  };

  var nativeFetch = window.fetch.bind(window);
  window.fetch = function (input, init) {
    if (typeof input === "string") {
      input = rewriteServerURL(input);
    }
    return nativeFetch(input, init);
  };

  window.alert = function () {
    console.log.apply(console, ["[White Raven]"].concat(Array.prototype.slice.call(arguments)));
  };

  function loadScript(source) {
    return new Promise(function (resolve, reject) {
      var script = document.createElement("script");
      script.src = devResource(source);
      script.onload = resolve;
      script.onerror = function () {
        reject(new Error("Could not load " + source));
      };
      document.head.appendChild(script);
    });
  }

  function addStylesheet(source) {
    var link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = devResource(source);
    document.head.appendChild(link);
  }

  function localData(key, value) {
    var storageKey = storagePrefix + "setting." + key;
    if (arguments.length > 1) {
      localStorage.setItem(storageKey, String(value));
      return value;
    }
    var result = localStorage.getItem(storageKey);
    return result === null ? undefined : result;
  }

  function browserLanguage() {
    var language = (navigator.language || "en").toLowerCase().split("-")[0];
    return ["bg", "hr", "en", "hu", "es", "sk", "it"].indexOf(language) >= 0 ? language : "en";
  }

  window.sf = {
    core: {
      getEnvValue: function (key) {
        return key === "lang" ? browserLanguage() : "";
      },
      localData: localData,
      loadJS: function (path, callback) {
        loadScript("/widget/" + path.replace(/^\/+/, "")).then(function () {
          if (callback) callback();
        }).catch(function (error) {
          setStatus(error.message, "error");
          console.error(error);
        });
      },
      exit: function () {
        setStatus("Widget exit requested (ignored in browser mode)");
      }
    },
    scene: {
      show: function (name, data) {
        var scene = scenes[name];
        if (scene && typeof scene.handleShow === "function") {
          scene.handleShow(data);
        }
      },
      hide: function (name, data) {
        var scene = scenes[name];
        if (scene && typeof scene.handleHide === "function") {
          scene.handleHide(data);
        }
      },
      focus: function (name, data) {
        if (focusedScene && focusedScene !== name) {
          var previous = scenes[focusedScene];
          if (previous && typeof previous.handleBlur === "function") previous.handleBlur();
        }
        focusedScene = name;
        var scene = scenes[name];
        if (scene && typeof scene.handleFocus === "function") scene.handleFocus(data);
      },
      get: function (name) {
        return scenes[name];
      },
      getFocused: function () {
        return focusedScene;
      }
    },
    key: {
      LEFT: 37,
      UP: 38,
      RIGHT: 39,
      DOWN: 40,
      ENTER: 13,
      RETURN: 8,
      EXIT: 81,
      RED: 403,
      GREEN: 404,
      YELLOW: 405,
      BLUE: 406,
      TOOLS: 84,
      INFO: 73,
      PLAY: 415,
      PAUSE: 19,
      STOP: 413,
      VOL_UP: 447,
      VOL_DOWN: 448,
      MUTE: 449,
      CH_UP: 427,
      preventDefault: function () {
        if (currentKeyEvent) currentKeyEvent.preventDefault();
      }
    }
  };

  window.Common = {
    API: {
      Widget: function () {
        this.putInnerHTML = function (element, html) {
          if (element) element.innerHTML = html;
        };
        this.sendReadyEvent = function () {};
      }
    }
  };

  window.curWidget = { id: "WhiteRavenBrowser" };

  window.FileSystem = function () {};
  window.FileSystem.prototype.isValidCommonPath = function (path) {
    return localStorage.getItem(storagePrefix + "dir." + path) === "1" ? 1 : 0;
  };
  window.FileSystem.prototype.createCommonDir = function (path) {
    localStorage.setItem(storagePrefix + "dir." + path, "1");
    return 1;
  };
  window.FileSystem.prototype.openCommonFile = function (path, mode) {
    var key = storagePrefix + "file." + path;
    if (mode === "r" && localStorage.getItem(key) === null) return null;
    return {
      readAll: function () { return localStorage.getItem(key) || ""; },
      writeAll: function (value) {
        var existing = mode === "a" ? localStorage.getItem(key) || "" : "";
        localStorage.setItem(key, existing + value);
      }
    };
  };
  window.FileSystem.prototype.closeCommonFile = function () {};
  window.FileSystem.prototype.deleteCommonFile = function (path) {
    localStorage.removeItem(storagePrefix + "file." + path);
    return 1;
  };

  window.SRect = function (left, top, width, height) {
    this.left = left;
    this.top = top;
    this.width = width;
    this.height = height;
  };

  function installPluginStubs() {
    var server = new URL(window.WHITE_RAVEN_SERVER_URL);
    var network = document.getElementById("networkplugin");
    network.GetActiveType = function () { return 0; };
    network.GetIP = function () { return server.hostname; };

    var filePlugin = document.getElementById("pluginFileSystem");
    filePlugin.IsExistedPath = function () { return 0; };

    var volume = 50;
    var muted = false;
    var audio = document.getElementById("pluginAudio1");
    audio.Open = function () { return true; };
    audio.Execute = function (command, value) {
      if (command === "SetVolumeWithKey") {
        volume = Math.max(0, Math.min(100, volume + (value === 0 ? 1 : -1)));
        muted = false;
      }
      if (command === "SetUserMute") muted = Boolean(value);
      if (command === "GetVolume") return muted ? 0 : volume;
      window.webapis.avplay.setVolume(volume / 100, muted);
      return true;
    };
  }


  window.webapis = { avplay: window.createBrowserAVPlay(rewriteServerURL) };

  function jquery(selector) {
    var elements = typeof selector === "string" ? document.querySelectorAll(selector) : [selector];
    return {
      sfList: function () { return this; },
      animate: function (properties) {
        Array.prototype.forEach.call(elements, function (element) {
          Object.keys(properties).forEach(function (key) {
            element.style[key] = typeof properties[key] === "number" ? properties[key] + "px" : properties[key];
          });
        });
        return this;
      }
    };
  }

  jquery.ajax = function (options) {
    var controller = new AbortController();
    var timer = options.timeout ? setTimeout(function () { controller.abort(); }, options.timeout) : null;
    var request = window.fetch(rewriteServerURL(options.url), {
      method: options.type || "GET",
      signal: controller.signal
    }).then(function (response) {
      if (!response.ok) throw new Error("HTTP " + response.status);
      return options.dataType === "json" ? response.json() : response.text();
    }).then(function (data) {
      if (options.success) options.success(data);
      return data;
    }).catch(function (error) {
      if (options.error) options.error(null, error.name, error);
    }).finally(function () {
      if (timer) clearTimeout(timer);
      if (options.complete) options.complete();
    });
    request.abort = function () { controller.abort(); };
    return request;
  };
  window.$ = jquery;

  function mapKey(event) {
    var key = event.key.toLowerCase();
    if (key === "arrowleft") return sf.key.LEFT;
    if (key === "arrowup") return sf.key.UP;
    if (key === "arrowright") return sf.key.RIGHT;
    if (key === "arrowdown") return sf.key.DOWN;
    if (key === "enter") return sf.key.ENTER;
    if (key === "backspace") return sf.key.RETURN;
    if (key === " ") return sf.key.TOOLS;
    if (key === "a") return sf.key.RED;
    if (key === "b") return sf.key.GREEN;
    if (key === "c") return sf.key.YELLOW;
    if (key === "d") return sf.key.BLUE;
    if (key === "+" || key === "=") return sf.key.VOL_UP;
    if (key === "-") return sf.key.VOL_DOWN;
    if (key === "m") return sf.key.MUTE;
    return null;
  }

  document.addEventListener("keydown", function (event) {
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    if (event.key.toLowerCase() === "f") {
      event.preventDefault();
      if (!document.documentElement.requestFullscreen) return;
      var fullscreen = document.fullscreenElement ? document.exitFullscreen() : document.documentElement.requestFullscreen();
      if (fullscreen && fullscreen.catch) fullscreen.catch(console.error);
      return;
    }
    var keyCode = mapKey(event);
    var scene = scenes[focusedScene];
    if (keyCode !== null && scene && typeof scene.handleKeyDown === "function") {
      if (keyCode === sf.key.ENTER && window.webapis && window.webapis.avplay && window.webapis.avplay.restoreBrowserAudio) {
        window.webapis.avplay.restoreBrowserAudio();
      }
      currentKeyEvent = event;
      scene.handleKeyDown(keyCode);
      currentKeyEvent = null;
    }
  });

  async function loadWidget() {
    installPluginStubs();
    var response = await nativeFetch(devResource("/widget/app.json"));
    if (!response.ok) throw new Error("Could not load widget/app.json");
    var application = await response.json();

    application.files.forEach(function (path) { addStylesheet("/widget/" + path); });
    application.scenes.forEach(function (name) {
      addStylesheet("/widget/app/stylesheets/720p/" + name + ".css");
    });

    var fragments = await Promise.all(application.scenes.map(function (name) {
      return nativeFetch(devResource("/widget/app/htmls/" + name + ".html")).then(function (result) { return result.text(); });
    }));
    document.getElementById("scene-root").innerHTML = fragments.join("\n");

    await loadScript("/widget/app/playback-tracks.js");
    for (var i = 0; i < application.scenes.length; i++) {
      await loadScript("/widget/app/scenes/" + application.scenes[i] + ".js");
    }

    // Samsung's app framework has the active language bundle available before
    // it initializes every scene. Reproduce that ordering explicitly.
    var initialLanguage = localData("interface") || browserLanguage();
    await loadScript("/widget/lang/" + initialLanguage + ".js");

    application.scenes.forEach(function (name) {
      var Constructor = window["Scene" + name];
      if (typeof Constructor !== "function") throw new Error("Missing scene constructor: Scene" + name);
      scenes[name] = new Constructor();
    });
    application.scenes.forEach(function (name) {
      if (typeof scenes[name].initialize === "function") scenes[name].initialize();
    });

    await loadScript("/widget/app/init.js");
    window.onStart();
    setStatus("Widget loaded; checking " + window.WHITE_RAVEN_SERVER_URL + "…");

    try {
      var about = await window.fetch(window.WHITE_RAVEN_SERVER_URL + "/api/v0/about");
      if (!about.ok) throw new Error("HTTP " + about.status);
      var result = await about.json();
      setStatus(result.message || "Connected", "ok");
    } catch (error) {
      setStatus("Server unavailable: " + error.message, "error");
    }
  }

  loadWidget().catch(function (error) {
    console.error(error);
    setStatus("Widget failed to load: " + error.message, "error");
  });
})();
