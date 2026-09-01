function SceneLoadSubtitle() {}

SceneLoadSubtitle.prototype.initialize = function () {
    this.pos = 0;
    this.waiting = false;
    this.request = (this.request || 0) + 1;
    this.prevtop = document.getElementById('OverlayPlayerMenuInfo').style.top;
    this.prevdata = document.getElementById('playermenuinfo').innerHTML;
}

SceneLoadSubtitle.prototype.handleShow = function (data) {
    this.initialize();
    this.caller = data.caller;
    
    document.getElementById('OverlaySubtitleMenu').style.visibility = "hidden";

    var filelist = document.getElementById('filelist');
    filelist.style.top = 'initial';
    widgetAPI.putInnerHTML(filelist, "");

    this.entries = sf.scene.get('PlayerPage').getSubtitleChoices();
    for (var i = 0; i < this.entries.length; i++) {
        var listitem = document.createElement('li');
        var aitem = document.createElement('a');
        aitem.className = i === 0 ? 'active' : '';
        aitem.textContent = (this.entries[i].selected ? '✓ ' : '') + this.entries[i].label;
        listitem.appendChild(aitem);
        filelist.appendChild(listitem);
    }
    var rows = Math.max(1, Math.min(5, this.entries.length));
    document.getElementById('OverlayLoadSubtitleMenu').style.height = CSSPixels(rows * 46);
    document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels((rows - 1) * 46 - 54);
    document.getElementById('OverlayLoadSubtitleMenu').style.visibility = 'visible';
    document.getElementById('playermenuinfo').textContent = this.entries.length ? '1 / ' + this.entries.length : '';
    document.getElementById('OverlayPlayerMenuInfo').style.visibility = 'visible';
    if (!this.entries.length) {
        filelist.textContent = subtitleNotFoundText[lang];
        this.waiting = true;
    }
}

SceneLoadSubtitle.prototype.handleHide = function (data) {
    this.request++;
    data = data || {caller: this.caller};
    document.getElementById('OverlayLoadSubtitleMenu').style.visibility = "hidden";
    document.getElementById('OverlayPlayerMenuInfo').style.top = CSSPixels(this.prevtop);
    widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), this.prevdata);
      
    if (data.caller == "SubtitleMenu") {
        sf.scene.get('SubtitleMenu').handleShow();
    } else if (data.caller == "SubtitleSearch") {
        document.getElementById('OverlayPlayerMenuInfo').style.visibility = "hidden";
    }
}

SceneLoadSubtitle.prototype.handleFocus = function () {};

SceneLoadSubtitle.prototype.handleBlur = function () {};

SceneLoadSubtitle.prototype.handleKeyDown = function (keyCode) {
    currentkeytime = new Date();
    if (currentkeytime - lastkeytime > keytimeout) {
        lastkeytime = currentkeytime;
        switch (keyCode) {
            case sf.key.ENTER:
                if (this.waiting == false) {
        	        var filelist = document.getElementById('filelist').getElementsByTagName("li");
                    for(var i=0; i<filelist.length; i++) {
                        if (filelist[i].children[0].className == 'active') {
                            this.selectEntry(i, filelist[i].children[0]);
                            break;
                        }
                    }
                }
    	        break;
            case sf.key.RETURN:
            	sf.key.preventDefault();
                if (this.caller == "SubtitleMenu") {
                	sf.scene.hide('LoadSubtitle', {caller: "SubtitleMenu"});
                	sf.scene.focus('SubtitleMenu');
                } else if (this.caller == "SubtitleSearch") {
                    sf.scene.hide('LoadSubtitle', {caller: "SubtitleSearch"});
                    sf.scene.focus('SubtitleSearch');
                }
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
                var filelist = document.getElementById('filelist').getElementsByTagName("li");
                for(var i=0; i<filelist.length; i++) {
                    if (filelist[i].children[0].className == 'active') {
                        filelist[i].children[0].className = "";
                        if (this.pos > 0) {
                            this.pos--;
                            i--;
                        } else {
                            if (i > 0) {
                                var nameul = document.getElementById('filelist');
                                nameul.style.top = CSSPixels(nameul.offsetTop + 46);
                                i--;
                            }
                        }
                        filelist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + filelist.length);
                    }
                }
                break;
            case sf.key.DOWN:
                var filelist = document.getElementById('filelist').getElementsByTagName("li");
                for(var i=0; i<filelist.length; i++) {
                    if (filelist[i].children[0].className == 'active') {
                        filelist[i].children[0].className = "";
                        if (this.pos < 4 && this.pos < (filelist.length - 1)) {
                            this.pos++;
                            i++;
                        } else {
                            if (i < filelist.length - 1) {
                                var nameul = document.getElementById('filelist');
                                nameul.style.top = CSSPixels(nameul.offsetTop - 46);
                                i++;
                            }
                        }
                        filelist[i].children[0].className = 'active';
                        widgetAPI.putInnerHTML(document.getElementById('playermenuinfo'), (i +  1) + " / " + filelist.length);
                    }
                }
                break;             
        }
    }
};

SceneLoadSubtitle.prototype.SetZIndex = function(state, number) {
    document.getElementById("OverlayVideoHeader").style.zIndex = number;
    document.getElementById("VideoHeader").style.zIndex = number;
    document.getElementById("OverlayVideoFooter").style.zIndex = number;
    document.getElementById("VideoFooter").style.zIndex = number;
    document.getElementById("waitscreen").style.visibility = state;
};

SceneLoadSubtitle.prototype.selectEntry = function(index, item) {
    var entry = this.entries[index];
    var player = sf.scene.get('PlayerPage');
    var request = ++this.request;
    this.waiting = true;
    item.textContent = subtitleLoadText[lang][0];
    var finished = function(selected) {
        if (request !== this.request) return;
        item.textContent = subtitleLoadText[lang][selected ? 1 : 2];
        setTimeout(function() {
            if (request !== this.request) return;
            var choices = player.getSubtitleChoices();
            var items = document.getElementById('filelist').getElementsByTagName('li');
            for (var i = 0; i < this.entries.length; i++) {
                var selected = false;
                for (var j = 0; j < choices.length; j++) {
                    if (choices[j].key === this.entries[i].key) selected = choices[j].selected;
                }
                items[i].children[0].textContent = (selected ? '✓ ' : '') + this.entries[i].label;
            }
            this.waiting = false;
        }.bind(this), 700);
    }.bind(this);
    if (entry.source === 'embedded') {
        var result = player.selectEmbeddedSubtitle(entry.index);
        if (result && typeof result.then === 'function') result.then(finished, function() { finished(false); });
        else finished(result);
    } else {
        player.DownloadAnotherSubtitle(entry.url, item, entry.label, finished);
    }
};
