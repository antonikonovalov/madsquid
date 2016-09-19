'use strict';

var startButton = document.getElementById('startButton');
var stopButton = document.getElementById('stopButton');
startButton.disabled = true;
stopButton.disabled = true

startButton.onclick = start;
stopButton.onclick = stop;

var localVideo = document.getElementById('localVideo');
var remoteVideo = document.getElementById('remoteVideo');


var userNameInput = document.getElementById('userName');

userNameInput.onkeyup = userChanges;

var videoCheckbox = document.getElementById('videoCheckbox');
var audioCheckbox = document.getElementById('audioCheckbox');

videoCheckbox.onclick = changeVideoTracks;
audioCheckbox.onclick = changeAudioTracks;

var usersSpan = document.getElementById('users');

function userChanges(evt) {
	startButton.disabled = !userNameInput.value
}

var pc;
var callee;
var localStream;
var signalingChannel = new SignalingChannel();

function trackOptions() {
    return {
        video: videoCheckbox.checked,
        audio: audioCheckbox.checked,
    };
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

    signalingChannel.start();

    pc = new RTCPeerConnection(null);
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
        remoteVideo.srcObject = e.stream;
        console.log('received remote stream');
    }
};

function stop() {
    userNameInput.disabled = false;
    startButton.disabled = false;
    stopButton.disabled = true;

    usersSpan.innerHTML = '';

    localVideo.srcObject = null;
    remoteVideo.srcObject = null;
    signalingChannel.stop();
    
    if (pc.signalingState!="closed") {
        pc.getLocalStreams().forEach(function(s) {
            s.getTracks().forEach(function(t) {t.stop()});
        });
        pc.close();
    }
    localStream = null;
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
        if (userNameInput.value) {
            this.socket.send(JSON.stringify({ callee: callee, content: obj }));
        }
    }

    this.stop = function() {
        if (this.socket) {
            this.socket.close();
            this.socket = null;
        }
    }

}

function callTo(user) {
    callee = user

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
	if (data.type == "offer") {
		console.log('offer received');

        callee = msg.from;

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
        usersSpan.innerHTML='';
        data.users.forEach(function(u) {
            if (u==userNameInput.value) return;
            var a = document.createElement('A');
            a.innerHTML = u;
            a.href = '#';
            a.onclick = function() {
                callTo(u);
                return false;
            }
            usersSpan.appendChild(a)
            usersSpan.appendChild(document.createTextNode(' '))
        })

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