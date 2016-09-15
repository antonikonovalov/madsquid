'use strict';

var callButton = document.getElementById('callButton');
var hangupButton = document.getElementById('hangupButton');
var startButton = document.getElementById('startButton');
var stopButton = document.getElementById('stopButton');
callButton.disabled = true;
hangupButton.disabled = true;
startButton.disabled = true;
stopButton.disabled = true

callButton.onclick = call;
hangupButton.onclick = hangup;
startButton.onclick = start;
stopButton.onclick = stop;

// var startTime;
var localVideo = document.getElementById('localVideo');
var remoteVideo = document.getElementById('remoteVideo');


var userNameInput = document.getElementById('userName');
var userCalleeInput = document.getElementById('userCallee');

userCalleeInput.disabled = true

userNameInput.onkeyup = userChanges;
userCalleeInput.onkeyup = calleeChanges;

function userChanges(evt) {
	startButton.disabled = !userNameInput.value
}

function calleeChanges(evt) {
    callButton.disabled = !(userCalleeInput.value && started)
}

var pc;
var callee;
var started=false
var signalingChannel = new SignalingChannel()

function gotStream(stream) {
  console.log('Received local stream');
  localVideo.srcObject = stream;
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

    // let the "negotiationneeded" event trigger offer generation
    pc.onnegotiationneeded = function () {
        console.log('Event "onnegotiationneeded"')
        pc.createOffer().then(function (offer) {
            pc.setLocalDescription(offer);
        })
        .then(function () {
            // send the offer to the other peer
            signalingChannel.send(pc.localDescription);
        })
        .catch(logError);
    };

    pc.onaddstream = gotRemoteStream;

    console.log('Requesting local stream');
    navigator.mediaDevices.getUserMedia({ audio: true, video: true })
    .then(gotStream)
    .catch(logError);
};

function stop() {
    userNameInput.disabled = false;
    startButton.disabled = false;
    stopButton.disabled = true;

    userCalleeInput.disabled = true;

    started = false;

    localVideo.srcObject = null;
    signalingChannel.stop();
}

function SignalingChannel() {
	this.start = function() {
		// this.getMessages()
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
            this.socket.send(JSON.stringify({ callee: callee, content: JSON.stringify(obj) }));
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
    hangupButton.disabled = false;
    callee = userCalleeInput.value

    // get a local stream, show it in a self-view and add it to be sent
    navigator.mediaDevices.getUserMedia({ audio: true, video: true })
    .then (function (stream) {
        pc.addStream(stream);
    })
    .catch(logError);
}

function hangup() {
    pc.close();
    hangupButton.disabled = true;
}

function parseMsg(msg) {
    var data = JSON.parse(msg.content)
    if (!data) return
	if (data.type == "offer") {
		console.log('offer received');

        hangupButton.disabled = false;
        callee = msg.from;

        pc.setRemoteDescription(new RTCSessionDescription(data))
        .then(function() {
            return navigator.mediaDevices.getUserMedia({ audio: true, video: true })
        }).then(function (stream) {
            return pc.addStream(stream)
        }).then(function() {
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

function gotRemoteStream(e) {
  remoteVideo.srcObject = e.stream;
  console.log('received remote stream');
}

function logError(error) {
    console.log(error.name + ": " + error.message);
}