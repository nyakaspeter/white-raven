function SceneSubtitleMenu() {}

SceneSubtitleMenu.prototype.initialize = function () {
    this.pos = 0;
    this.waiting = false;
}

SceneSubtitleMenu.prototype.handleShow = function () {
    this.initialize();
    
    var subtitlemenulist = document.getElementById('subtitlemenulist');
    subtitlemenulist.style.top = 'initial';
    widgetAPI.putInnerHTML(subtitlemenulist, "");

    var player = sf.scene.get('PlayerPage');
    var choices = player.getSubtitleChoices();
    var nativeState = player.Player.tracks;
    var hasSelection = player.Player.externalSubtitleURL || (nativeState && nativeState.subtitleIndex >= 0);
    this.entries = [];
    if (hasSelection) {
        var active = nativeState && nativeState.subtitleIndex >= 0 ? nativeState.subtitleActive : player.Player.subtitleState === 1;
        this.entries.push({name: active ? 'hide' : 'show', label: active ? subtitleShowText[lang] : subtitleHiddenText[lang]});
        this.entries.push({name: 'sync', label: subtitleMenuText[lang][1]});
    }
    if (choices.length > 0) this.entries.push({name: 'load', label: subtitleMenuText[lang][2]});
    if (hasSelection) this.entries.push({name: 'style', label: subtitleMenuText[lang][4]});
    if (saveSettings['issubtitleenabled'] == "true") this.entries.push({name: 'search', label: subtitleMenuText[lang][3]});

    if (this.entries.length > 0) {
        var menulength = this.entries.length;

        for(var i=0; i<menulength; i++) {    
            var listitem = document.createElement('li');
            var aitem = document.createElement('a');
            if (i == 0) {
                aitem.className = "active";
            } else {
                aitem.className = "";
            }

            aitem.textContent = this.entries[i].label;
            listitem.appendChild(aitem);
            subtitlemenulist.appendChild(listitem);
        }

        if (menulength > 0) {
            if (menulength < 7) {
                document.getElementById('OverlaySubtitleMenu').style.height = CSSPixels(((menulength - 1) * 46) + 46);
                document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels(((menulength - 1) * 46) + 56);
            } else {
                document.getElementById('OverlaySubtitleMenu').style.height = CSSPixels(((7 - 1) * 46) + 46);
                document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels(((7 - 1) * 46) + 56);
            }
            document.getElementById('OverlaySubtitleMenu').style.visibility = "visible";
            widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), "1 / " + menulength);
            document.getElementById('OverlayPlayerMenuInfo').style.visibility = "visible";
        }
    } else {
        // No tracks are available and online subtitle search is disabled.
        this.waiting = true;
        widgetAPI.putInnerHTML(document.getElementById('mediapageinfo'), "<DL><DT>" + subtitleMustEnabledText[lang] + "</DT><DD></DD><SPAN></SPAN></DL>");
        document.getElementById('OverlayMediaPage').style.height = CSSPixels(document.getElementById('mediapageinfo').offsetHeight);
        document.getElementById('OverlayMediaPage').style.visibility = "visible";
    }
}

SceneSubtitleMenu.prototype.handleHide = function () {
    this.selectionRequest = (this.selectionRequest || 0) + 1;
    document.getElementById('OverlaySubtitleMenu').style.visibility = 'hidden';
    document.getElementById('OverlayPlayerMenuInfo').style.visibility = 'hidden';
    document.getElementById('OverlayMediaPage').style.visibility = 'hidden';
};

SceneSubtitleMenu.prototype.handleFocus = function () {};

SceneSubtitleMenu.prototype.handleBlur = function () {};

SceneSubtitleMenu.prototype.handleKeyDown = function (keyCode) {
    currentkeytime = new Date();
    if (currentkeytime - lastkeytime > keytimeout) {
        lastkeytime = currentkeytime;
        switch (keyCode) {
            case sf.key.ENTER:
    	        if (this.waiting == false) {
                    var subtitlemenulist = document.getElementById('subtitlemenulist').getElementsByTagName("li");
                    for(var i=0; i<subtitlemenulist.length; i++) {
                        if (subtitlemenulist[i].children[0].className == 'active') {

                            var entry = this.entries[i];
                            var itemname = entry.name;
                            break;
                        }
                    }

        	        switch (itemname) {
        	        	case "hide":
                            sf.scene.get('PlayerPage').menuStopSubtitle();
                            subtitleMenuText['name'][0] = 'show';
                            subtitleMenuText[lang][0] = subtitleHiddenText[lang];
                            this.handleShow();
        		        	break;
                        case "show":
                            var nativeState = sf.scene.get('PlayerPage').Player.tracks;
                            if (nativeState && nativeState.subtitleIndex >= 0) {
                                this.selectEmbedded(nativeState.subtitleIndex, subtitlemenulist[i].children[0]);
                                break;
                            }
                            this.waiting = true;
                            sf.scene.get('PlayerPage').DownloadAnotherSubtitle(sf.scene.get('PlayerPage').Player.externalSubtitleURL, subtitlemenulist[0].children[0], subtitlemenulist[0].children[0].innerHTML, function(state, menuelement, menuHTML) {
                                if (state == true) {
                                    widgetAPI.putInnerHTML(menuelement, subtitleLoadText[lang][1]); // ok text
                                    setTimeout(function() {
                                        subtitleMenuText['name'][0] = 'hide';
                                        subtitleMenuText[lang][0] = subtitleShowText[lang];
                                        if (sf.scene.getFocused() == "SubtitleMenu") {
                                            this.handleShow();
                                        }
                                    }.bind(this), 1000);
                                } else {
                                    widgetAPI.putInnerHTML(menuelement, subtitleLoadText[lang][2]); // error text
                                    setTimeout(function() {
                                        widgetAPI.putInnerHTML(menuelement, menuHTML);
                                        this.waiting = false;
                                    }.bind(this), 1000);
                                }
                            }.bind(this));
                            break;
        	        	case "sync":
                            sf.scene.show('SubtitleSync');
                            sf.scene.focus('SubtitleSync');
        		        	break;
        	        	case "load":
        	        		sf.scene.show('LoadSubtitle', {caller: "SubtitleMenu"});
        	        		sf.scene.focus('LoadSubtitle');
        		        	break;
                        case "search":
                            sf.scene.show('SubtitleSearch', {caller: "SubtitleMenu"});
                            sf.scene.focus('SubtitleSearch');
                            break; 
                        case "style":
                            sf.scene.show('SubtitleStyle');
                            sf.scene.focus('SubtitleStyle');
                            break;  
        	        }
                }
    	        break;
            case sf.key.RETURN:
            	sf.key.preventDefault();
            	sf.scene.hide('SubtitleMenu');
            	sf.scene.focus('PlayerPage');
            	break;
            case sf.key.EXIT:
                sf.key.preventDefault();
                StartStopWRServer("stop");
                sf.core.exit(false);
                break;
            // Comment out this case for real time Player Menu Auto Hide testing
            /*case sf.key.CH_UP:
                sf.scene.get('PlayerPage').playerMenuAutoHideTest();
                break;*/
        }
    }
    lastkeytime = currentkeytime;
    sf.key.preventDefault();
    if (this.waiting == false) {
        switch (keyCode) {
            case sf.key.UP:
                var subtitlemenulist = document.getElementById('subtitlemenulist').getElementsByTagName("li");
                for(var i=0; i<subtitlemenulist.length; i++) {
                    if (subtitlemenulist[i].children[0].className == 'active') {
                        subtitlemenulist[i].children[0].className = "";
                        if (this.pos > 0) {
                            this.pos--;
                            i--;
                        } else {
                            if (i > 0) {
                                var nameul = document.getElementById('subtitlemenulist');
                                nameul.style.top = CSSPixels(nameul.offsetTop + 46);
                                i--;
                            }
                        }
                        subtitlemenulist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + subtitlemenulist.length);
                    }
                }
                break;
            case sf.key.DOWN:
                var subtitlemenulist = document.getElementById('subtitlemenulist').getElementsByTagName("li");
                for(var i=0; i<subtitlemenulist.length; i++) {
                    if (subtitlemenulist[i].children[0].className == 'active') {
                        subtitlemenulist[i].children[0].className = "";
                        if (this.pos < 6 && this.pos < (subtitlemenulist.length - 1)) {
                            this.pos++;
                            i++;
                        } else {
                            if (i < subtitlemenulist.length - 1) {
                                var nameul = document.getElementById('subtitlemenulist');
                                nameul.style.top = CSSPixels(nameul.offsetTop - 46);
                                i++;
                            }
                        }
                        subtitlemenulist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + subtitlemenulist.length);
                    }
                }
                break;             
        }
    }
};

SceneSubtitleMenu.prototype.SetZIndex = function(state, number) {
    document.getElementById("OverlayVideoHeader").style.zIndex = number;
    document.getElementById("VideoHeader").style.zIndex = number;
    document.getElementById("OverlayVideoFooter").style.zIndex = number;
    document.getElementById("VideoFooter").style.zIndex = number;
    document.getElementById("waitscreen").style.visibility = state;
};

SceneSubtitleMenu.prototype.selectEmbedded = function(index, item) {
    this.waiting = true;
    var request = this.selectionRequest = (this.selectionRequest || 0) + 1;
    var finished = function(selected) {
        if (request !== this.selectionRequest) return;
        this.waiting = false;
        if (selected) this.handleShow();
        else item.textContent = playbackTrackText[lang][2];
    }.bind(this);
    var result = sf.scene.get('PlayerPage').selectEmbeddedSubtitle(index);
    if (result && typeof result.then === 'function') result.then(finished, function() { finished(false); });
    else finished(result);
};
