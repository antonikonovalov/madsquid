'use strict';

var signalingMethod = "websocket"; // websocket | post

var startButton = document.getElementById('startButton');
var stopButton = document.getElementById('stopButton');
var testButton = document.getElementById('testButton');
startButton.disabled = false;
stopButton.disabled = true
testButton.disabled = true

startButton.onclick = start;
stopButton.onclick = stop;
testButton.onclick = test;

var localVideo = document.getElementById('localVideo');

var userNameInput = document.getElementById('userName');

userNameInput.onkeyup = userChanges;

var videoCheckbox = document.getElementById('videoCheckbox');
var audioCheckbox = document.getElementById('audioCheckbox');

videoCheckbox.onclick = changeVideoTracks;
audioCheckbox.onclick = changeAudioTracks;

var usersDiv = document.getElementById('users');

function userChanges(evt) {
    startButton.disabled = !userNameInput.value
}

var pcs = {};
var callee;
var localStream;
var signalingChannel = new SignalingChannel();

function trackOptions() {
    return {
        video: videoCheckbox.checked,
        audio: audioCheckbox.checked,
    };
}

function iceServers() {
    return [
        {url:'stun:stun.l.google.com:19302'}
    ]
}

function changeVideoTracks() {
    if (!localStream) return;
    changeTracks(videoCheckbox.checked, localStream.getVideoTracks())
}

function changeAudioTracks() {
    if (!localStream) return;
    changeTracks(audioCheckbox.checked, localStream.getAudioTracks())
}

function changeTracks(flag, tracks) {
    tracks.forEach(function(t) { t.enabled=flag; })
}

function gotStream(stream) {
  console.log('Received local stream');
  localStream = stream;
  changeVideoTracks();
  changeAudioTracks();
  localVideo.srcObject = localStream;
}


function start() {
    userNameInput.disabled = true;
    startButton.disabled = true;
    stopButton.disabled = false;
    testButton.disabled = false;

    localVideo.id = userNameInput.value

    signalingChannel.start();
};

function stop() {
    userNameInput.disabled = false;
    startButton.disabled = false;
    stopButton.disabled = true;
    testButton.disabled = true;

    usersDiv.innerHTML = '';

    localVideo.srcObject = null;
    signalingChannel.stop();

    Object.keys(pcs).map(function(key, ind){
        if (pcs[key].signalingState!="closed") {
            pcs[key].getLocalStreams().forEach(function(s) {
                s.getTracks().forEach(function(t) {t.stop()});
            });
            pcs[key].close();
        }
    })
    
    localStream = null;
}

function test() {
    testButton.disabled = true;

    var elems = Array.from(document.getElementsByClassName("video-label"));
    elems.forEach(function(elem, index){
        setTimeout(function(){elem.click();}, 60*1000*index+Math.log(index+1));
    });

    setTimeout(function(){stopButton.click(); setTimeout(function(){alert("FINISHED");}, 1);}, 60*1000*elems.length+Math.log(elems.length));
}

function SignalingChannel() {
	this.start = function() {
        this.socket = new WebSocket("wss://"+window.location.host+"/ws?user="+encodeURIComponent(userNameInput.value));
        this.socket.onopen = function() {
            console.log("websocket connected");
        };
        this.socket.onclose = function(event) {
            console.log('websocket closed. code: ' + event.code + ', reason: ' + event.reason);
            stop();
        };
        this.socket.onmessage = function(event) {
            parseMsg(JSON.parse(event.data));
        };
        this.socket.onerror = function(error) {
            console.log("websocket error " + error.message);
        };
	};

    this.send = function(obj) {
        if (signalingMethod=='websocket')
            this.sendWebSocket(obj)
        else
            this.sendPost(obj)
    }

    this.sendWebSocket = function(obj) {
        this.socket.send(JSON.stringify({ for: callee, content: obj }));
    }

    this.sendPost = function(obj) {
        var client = new XMLHttpRequest();
        client.onload = function() {
          if (this.status < 200 || this.status >= 300) {
            logError(this.statusText);
          }
        };
        client.onerror = function () {
          logError(this.statusText);
        };

        client.open("POST", "/messages", true);
        client.setRequestHeader('Content-type', 'application/json; charset=utf-8');
        client.send(JSON.stringify({
            for: callee,
            from: userNameInput.value,
            content: obj
        }));
    }

    this.stop = function() {
        if (this.socket) {
            this.socket.close();
            this.socket = null;
        }
    }
}

function newPC(callee) {

    var pc = new RTCPeerConnection({iceServers:iceServers()});
    // send any ice candidates to the other peer
    pc.onicecandidate = function (evt) {
        console.log('Event "onicecandidate"');
        signalingChannel.send(evt.candidate);
    };

    pc.onnegotiationneeded = function () {
        console.log('Event "onnegotiationneeded"')
        pc.createOffer().then(function (offer) {
            pc.setLocalDescription(offer);
            signalingChannel.send(offer);
        })
        .catch(logError);
    };

    pc.onaddstream = function(e) {
        var remoteVideo = document.getElementById(callee);
        remoteVideo.srcObject = e.stream;
        console.log('received remote stream');
    }

    pcs[callee] = pc;

    return pc;
}

function callTo(user) {
    callee = user


    var pc = pcs[callee] || newPC(callee);
    // get a local stream, show it in a self-view and add it to be sent
    navigator.mediaDevices.getUserMedia({video:true, audio:true})
    .then (function (stream) {
        gotStream(stream);
        pc.addStream(stream);
    })
    .catch(logError);
}

function parseMsg(msg) {
    var data = msg.content
    if (!data) return

    callee = msg.from;
    var pc = pcs[callee] || newPC(callee);

    if (data.type == "offer") {
        console.log('offer received');

        pc.setRemoteDescription(new RTCSessionDescription(data))
        .then(function() {
            if (pc.getLocalStreams().length<1) {
                navigator.mediaDevices.getUserMedia({video:true, audio:true}).then(function (stream) {
                    gotStream(stream);
                    pc.addStream(stream)
                })
                .catch(logError);
            }
            return pc.createAnswer()
        })
        .then(function (answer) {
            pc.setLocalDescription(answer);
            signalingChannel.send(answer);
        })
        .catch(logError);

	} else if (data.type == "answer") {
		console.log('answer received')
        pc.setRemoteDescription(new RTCSessionDescription(data))
        .catch(logError);

    } else if (data.type == "users") {
        console.log("get users: " + data.users)
        // usersDiv.innerHTML='';
        data.users.forEach(function(u) {
            if (u==userNameInput.value) return;

            var videotag = document.getElementById(u);
            console.log("videotag", videotag);
            if (videotag === null) {
                var container = document.createElement('div');
                container.className = "container";
                var video = document.createElement('video');
                video.id = u;
                video.autoplay = true;
                container.appendChild(video);
                var link = document.createElement('a');
                link.href = '#';
                link.className = 'video-label';
                link.innerHTML = '<b> call to '+u+'</b>';
                link.onclick = function() {
                    callTo(u);
                    return false;
                }
                container.appendChild(link);

                usersDiv.appendChild(container);
            }
        });

        Object.keys(pcs).map(function(key, ind){
            if (pcs[key].signalingState =="closed") {
                delete pcs[key];
            } else if (pcs[key].signalingState !="closed" && !data.users.includes(key)) {
                pcs[key].getLocalStreams().forEach(function(s) {
                    s.getTracks().forEach(function(t) {t.stop()});
                });
                pcs[key].close();
                delete pcs[key];
            }
        });

        Array.from(document.getElementsByClassName("container")).forEach(function(cont){
            if (!data.users.includes(cont.getElementsByTagName("video")[0].id)){
                cont.parentNode.removeChild(cont);
            }
        });
        
    } else {
    	console.log('candidate received');
        if (msg.content) {
    	   pc.addIceCandidate(new RTCIceCandidate(data)).catch(logError);
        }
    }
}

function logError(error) {
    console.log(error.name + ": " + error.message);
}