function SceneAudioMenu() {}

SceneAudioMenu.prototype.initialize = function () {
    this.pos = 0;
    this.waiting = false;
    this.request = (this.request || 0) + 1;
}

SceneAudioMenu.prototype.handleShow = function (streamcount) {
    this.initialize();
    this.tracks = sf.scene.get('PlayerPage').getAudioTracks();
    streamcount = this.tracks.length;
    
    document.getElementById('OverlayAudioMenu').style.visibility = "hidden";
    document.getElementById('OverlayPlayerMenuInfo').style.visibility = "hidden";

    var audiomenulist = document.getElementById('audiomenulist');
    audiomenulist.className = streamcount === 0 ? 'audio-menu-empty' : '';
    audiomenulist.style.top = 'initial';
    widgetAPI.putInnerHTML(audiomenulist, "");

    for(var i=0; i<streamcount; i++) {    
        var listitem = document.createElement('li');
        var aitem = document.createElement('a');
        if (i == 0) {
            aitem.className = "active";
        } else {
            aitem.className = "";
        }

        aitem.textContent = (this.tracks[i].selected ? '✓ ' : '') + PlaybackAudioTrackLabel(this.tracks[i], audioStreamText[lang][0]);
        
        listitem.appendChild(aitem);
        audiomenulist.appendChild(listitem);
    }

    if (streamcount === 0) {
        this.waiting = true;
        audiomenulist.textContent = playbackTrackText[lang][1];
        document.getElementById('OverlayAudioMenu').style.height = CSSPixels(92);
        document.getElementById('OverlayAudioMenu').style.visibility = 'visible';
    }
    if (streamcount > 0) {
        if (streamcount < 5) {
            document.getElementById('OverlayAudioMenu').style.height = CSSPixels(((streamcount - 1) * 46) + 46);
            document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels(((streamcount - 1) * 46) + 56);
        } else {
            document.getElementById('OverlayAudioMenu').style.height = CSSPixels(((5 - 1) * 46) + 46);
            document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels(((5 - 1) * 46) + 56);
        }
        document.getElementById('OverlayAudioMenu').style.visibility = "visible";
        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), "1 / " + streamcount);
        document.getElementById('OverlayPlayerMenuInfo').style.visibility = "visible";
    }    
}

SceneAudioMenu.prototype.handleHide = function () {
    this.request++;
    document.getElementById('OverlayAudioMenu').style.visibility = "hidden";
    document.getElementById('OverlayPlayerMenuInfo').style.visibility = "hidden";
};

SceneAudioMenu.prototype.handleFocus = function () {};

SceneAudioMenu.prototype.handleBlur = function () {};

SceneAudioMenu.prototype.handleKeyDown = function (keyCode) {
    currentkeytime = new Date();
    if (currentkeytime - lastkeytime > keytimeout) {
        lastkeytime = currentkeytime;
        switch (keyCode) {
            case sf.key.ENTER:
                if (this.waiting == false) {
        	        var audiomenulist = document.getElementById('audiomenulist').getElementsByTagName("li");
                    for(var i=0; i<audiomenulist.length; i++) {
                        if (audiomenulist[i].children[0].className == 'active') {
                            if (this.tracks[i].selected) break;
                            this.waiting = true;
                            var request = this.request;
                            var finished = function(selected) {
                                if (request !== this.request) return;
                                if (selected) {
                                    for (var j = 0; j < this.tracks.length; j++) {
                                        this.tracks[j].selected = j === i;
                                        audiomenulist[j].children[0].textContent = (j === i ? '✓ ' : '') + PlaybackAudioTrackLabel(this.tracks[j], audioStreamText[lang][0]);
                                    }
                                }
                                audiomenulist[i].children[0].textContent = selected ? audioStreamText[lang][1] : playbackTrackText[lang][2];
                                setTimeout(function(){
                                    if (request !== this.request) return;
                                    audiomenulist[i].children[0].textContent = (this.tracks[i].selected ? '✓ ' : '') + PlaybackAudioTrackLabel(this.tracks[i], audioStreamText[lang][0]);
                                    this.waiting = false;
                                }.bind(this), 700);
                            }.bind(this);
                            var selected = sf.scene.get('PlayerPage').setAudioStreamID(this.tracks[i].index);
                            if (selected && typeof selected.then === 'function') selected.then(finished, function() { finished(false); });
                            else finished(selected);
                            break;
                        }
                    }
                }
    	        break;
            case sf.key.RETURN:
            	sf.key.preventDefault();
            	sf.scene.hide('AudioMenu');
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
                var audiomenulist = document.getElementById('audiomenulist').getElementsByTagName("li");
                for(var i=0; i<audiomenulist.length; i++) {
                    if (audiomenulist[i].children[0].className == 'active') {
                        audiomenulist[i].children[0].className = "";
                        if (this.pos > 0) {
                            this.pos--;
                            i--;
                        } else {
                            if (i > 0) {
                                var nameul = document.getElementById('audiomenulist');
                                nameul.style.top = CSSPixels(nameul.offsetTop + 46);
                                i--;
                            }
                        }
                        audiomenulist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + audiomenulist.length);
                    }
                }
                break;
            case sf.key.DOWN:
                var audiomenulist = document.getElementById('audiomenulist').getElementsByTagName("li");
                for(var i=0; i<audiomenulist.length; i++) {
                    if (audiomenulist[i].children[0].className == 'active') {
                        audiomenulist[i].children[0].className = "";
                        if (this.pos < 4 && this.pos < (audiomenulist.length - 1)) {
                            this.pos++;
                            i++;
                        } else {
                            if (i < audiomenulist.length - 1) {
                                var nameul = document.getElementById('audiomenulist');
                                nameul.style.top = CSSPixels(nameul.offsetTop - 46);
                                i++;
                            }
                        }
                        audiomenulist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + audiomenulist.length);
                    }
                }
                break;             
        }
    }
};

SceneAudioMenu.prototype.SetZIndex = function(state, number) {
    document.getElementById("OverlayVideoHeader").style.zIndex = number;
    document.getElementById("VideoHeader").style.zIndex = number;
    document.getElementById("OverlayVideoFooter").style.zIndex = number;
    document.getElementById("VideoFooter").style.zIndex = number;
    document.getElementById("waitscreen").style.visibility = state;
};
