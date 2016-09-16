'use strict';

var callButton = document.getElementById('callButton');
var startButton = document.getElementById('startButton');
var stopButton = document.getElementById('stopButton');
callButton.disabled = true;
startButton.disabled = true;
stopButton.disabled = true

callButton.onclick = call;
startButton.onclick = start;
stopButton.onclick = stop;

var localVideo = document.getElementById('localVideo');
var remoteVideo = document.getElementById('remoteVideo');


var userNameInput = document.getElementById('userName');
var userCalleeInput = document.getElementById('userCallee');

userCalleeInput.disabled = true

userNameInput.onkeyup = userChanges;
userCalleeInput.onkeyup = calleeChanges;

var videoCheckbox = document.getElementById('videoCheckbox');
var audioCheckbox = document.getElementById('audioCheckbox');

videoCheckbox.onclick = changeVideoTracks;
audioCheckbox.onclick = changeAudioTracks;

function userChanges(evt) {
	startButton.disabled = !userNameInput.value
}

function calleeChanges(evt) {
    callButton.disabled = !(userCalleeInput.value && started)
}

var pc;
var callee;
var started=false;
var localStream;
var signalingChannel = new SignalingChannel();

function trackOptions() {
    return {
        video: videoCheckbox.checked,
        audio: audioCheckbox.checked,
    };
}

function changeVideoTracks() {
    changeTracks(videoCheckbox.checked, localStream.getVideoTracks())
}

function changeAudioTracks() {
    changeTracks(audioCheckbox.checked, localStream.getAudioTracks())
}

function changeTracks(flag, tracks) {
    if (!localStream) return;
    if (flag && tracks.length<1) {
        // add video stream
        navigator.mediaDevices.getUserMedia(trackOptions())
        .then (function (stream) {
            localStream = stream;
            // pc.getLocalStreams().forEach(function(s) {pc.removeStream(s)})
            var streams = pc.getLocalStreams();

            streams.forEach(function(s) {pc.removeStream(s)});
            pc.addStream(stream)
            
            localVideo.srcObject = stream
            
            streams.forEach(function(s) {
                s.getTracks().forEach(function(t) {t.stop()});
            });
        })
    } else if (!flag && tracks.length>0) {
        // remove all tracks from local stream
        tracks.forEach(function(t) { localStream.removeTrack(t); t.stop(); })
        localVideo.srcObject = localStream
    }
}

function gotStream(stream) {
  console.log('Received local stream');
  localStream = stream;
  localVideo.srcObject = localStream;
  callButton.disabled = !(userNameInput.value && userCalleeInput.value);
}

function start() {
    userNameInput.disabled = true;
    startButton.disabled = true;
    stopButton.disabled = false;

    userCalleeInput.disabled = false;

    started = true;

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
    // pc.ontrack = function(e) {
    //     remoteVideo.srcObject = e.streams[0];
    //     console.log('received remote tracks');
    // }
};

function stop() {
    userNameInput.disabled = false;
    startButton.disabled = false;
    stopButton.disabled = true;

    userCalleeInput.disabled = true;

    started = false;

    if (pc.signalingState!="closed") pc.close();

    localVideo.srcObject = null;
    remoteVideo.srcObject = null;
    signalingChannel.stop();
    pc.getLocalStreams().forEach(function(s) {
        s.getTracks().forEach(function(t) {t.stop()});
    });
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

function call() {
    callee = userCalleeInput.value

    // get a local stream, show it in a self-view and add it to be sent
    navigator.mediaDevices.getUserMedia(trackOptions())
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
                navigator.mediaDevices.getUserMedia(trackOptions()).then(function (stream) {
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

    } else {
    	console.log('candidate received');
        if (msg.content) {
    	   pc.addIceCandidate(data).catch(logError);
        }
    }
}

function logError(error) {
    console.log(error.name + ": " + error.message);
}