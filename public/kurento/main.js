'use strict';

var signalingMethod = "websocket"; // websocket | post

var audioCodec = "opus"
var videoCodec = "vp8"

var autoCallCheckbox = document.getElementById('autoCallCheckbox');


var firstPage = document.getElementById('first-page');
var roomPage = document.getElementById('room-page');
var joinButton = document.getElementById('joinButton');
var leaveButton = document.getElementById('leaveButton');
//var testButton = document.getElementById('testButton');
joinButton.disabled = false;
leaveButton.disabled = true;
leaveButton.disabled = true;
//stopButton.disabled = true
//testButton.disabled = true

joinButton.onclick = join;
leaveButton.onclick = leave;
/*
stopButton.onclick = stop;
testButton.onclick = test;
*/

var localVideo = document.getElementById('localVideo');
var title = document.getElementById('titleRoom');

var userNameInput = document.getElementById('userName');
var roomNameInput = document.getElementById('roomName');

userNameInput.onkeyup = inputChanges;
roomNameInput.onkeyup = inputChanges;

var videoCheckbox = document.getElementById('videoCheckbox');
var audioCheckbox = document.getElementById('audioCheckbox');
//var upstreamCheckbox = document.getElementById('upstreamCheckbox');

videoCheckbox.onclick = changeVideoTracks;
audioCheckbox.onclick = changeAudioTracks;
//upstreamCheckbox.onclick = changeUpstream;

var usersDiv = document.getElementById('users');

function inputChanges(evt) {
    joinButton.disabled = !(userNameInput.value||roomNameInput.value);
}



var pcs = {};
var callee;
var localStream;
var signalingChannel = new SignalingChannel();

//connect to websocket
signalingChannel.start();


var queue = {};
var queueRunner = {};

function putToQueue(callee, data) {
    var q = queue[callee] || new Array();
    if (!queueRunner[callee]) {
        q.push(data);
    } else {
        signalingChannel.send(data);
    }
}

function runQueue(callee) {
    var q = queue[callee];
    queueRunner[callee] = true;
    if (!q) return;

    q.forEach(function(data){
        signalingChannel.send(data);
    });
}


function trackOptions() {
    return {
        video: videoCheckbox.checked,
        audio: audioCheckbox.checked
    };
}

function ConfigPC() {
    return { iceServers: [{"urls":"stun:stun2.l.google.com:19302"}, {urls:"stun:stun.ekiga.net"}]}
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
  //changeVideoTracks();
  //changeAudioTracks();§
  localVideo.srcObject = localStream;
}


function join() {
    userNameInput.disabled = true;
    roomNameInput.disabled = true;
    joinButton.disabled = true;
    //stopButton.disabled = false;
    //testButton.disabled = false;

    localVideo.id = userNameInput.value;

    signalingChannel.send({
        cmd:"joinRoom",
        room: roomNameInput.value,
        user: userNameInput.value
    });
    title.innerText = 'room: '+roomNameInput.value + ', user: '+userNameInput.value;
    firstPage.style.display = "none";
    roomPage.style.display = "block";
    leaveButton.style.display = "block";
    leaveButton.disabled = false;
    callTo(userNameInput.value);


}

function leave() {



    Object.keys(pcs).map(function(key, ind){
        stopPC(key);
        if (key  !=  userNameInput.value) {
            document.getElementById('container-' + key).remove();
        }
    });

    signalingChannel.send({cmd:"leave"});

    userNameInput.disabled = false;
    roomNameInput.disabled = false;
    joinButton.disabled = false;

    firstPage.style.display = "block";
    roomPage.style.display = "none";

    localStream = null;
}


function SignalingChannel() {
	this.start = function() {
        this.socket = new WebSocket("wss://"+window.location.host+"/kurento");
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
        if (obj != null) {
        this.socket.send(JSON.stringify(obj));
        }
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

var sendOfferCfg = {offerToReceiveVideo: -1, offerToReceiveAudio: -1, voiceActivityDetection: true, iceRestart: false};
var recvOfferCfg = {offerToReceiveVideo: 1, offerToReceiveAudio: 1, voiceActivityDetection: true, iceRestart: false};

function newPC(callee) {

    var pc = new RTCPeerConnection(ConfigPC());
    // send any ice candidates to the other peer
    pc.onicecandidate = function (evt) {
        console.log('Event "onicecandidate"');
        if (evt.candidate != null) {
            signalingChannel.send({
                cmd: 'onIceCandidate',
                sender: callee,
                candidate: {candidate:evt.candidate}
            });
        }
    };

    pc.onnegotiationneeded = function () {
        console.log('Event "onnegotiationneeded"')
        var cfgOffer = sendOfferCfg;

        if (localStream != null) {
            cfgOffer = recvOfferCfg;
        }

        pc.createOffer(cfgOffer).then(function (offer) {
            pc.setLocalDescription(offer);
            signalingChannel.send({
                cmd: "receiveVideoFrom",
                sender: callee,
                sdpOffer: offer.sdp
            });
        }).catch(logError);
    };

    pc.onaddstream = function(e) {
        console.log('received remote stream');
        if (callee != userNameInput.value) {
            var remoteVideo = document.getElementById(callee);
            remoteVideo.srcObject = e.stream;
        }
    };

    pc.onclose = function () {
        if (callee != userNameInput.value) {
            var remoteVideo = document.getElementById('container-'+callee);
            if (remoteVideo != null) remoteVideo.remove();
        }
    };

    pcs[callee] = pc;

    return pc;
}

function callTo(user) {
    callee = user;

    var pc = pcs[callee] || newPC(callee);
    if (localStream != null) {
        pc.createOffer(recvOfferCfg).then(function (offer) {
            pc.setLocalDescription(offer);
            signalingChannel.send({
                cmd: "receiveVideoFrom",
                sender: callee,
                sdpOffer: offer.sdp
            });
        }).catch(logError);
    } else {
        // get a local stream, show it in a self-view and add it to be sent
        navigator.mediaDevices.getUserMedia({video : {
            mandatory : {
                maxWidth : 320,
                maxFrameRate : 15,
                minFrameRate : 15
            }
        }, audio: true}).then(function (stream) {
                gotStream(stream);
                pc.addStream(stream);
        }).catch(logError);
    }
}

function parseMsg(msg) {
    var data = msg;
    if (!data) return;


    if (data.error !== undefined) {
        console.error(data);
        return leave();
    }

    if (data.cmd === undefined) return;


    switch(data.cmd) {
        case 'receiveVideoAnswer':
            console.log('answer received');
            var pc = pcs[data.name];
            runQueue(data.name);
            pc.setRemoteDescription(new RTCSessionDescription({"type":"answer","sdp":data.sdpAnswer})).catch(logError);
            break;

        case 'iceCandidate':
            console.log('iceCandidate');
            var pc = pcs[data.name];
            pc.addIceCandidate(new RTCIceCandidate(data.candidate)).catch(logError);
            break;

        case  'participantLeaved':
            var video = document.getElementById('container-'+data.name);
            stopPC(data.name);

            if (video != null) video.remove();
            break;

        case 'newParticipantArrived':
            if (data.name==userNameInput.value) return;

            var videotag = document.getElementById(data.name);

            console.log("videotag", videotag);
            if (videotag === null) {
                CreateVideo(data.name);
            }
            break;

        case 'existingParticipants':
            console.log('existingParticipants');
            console.log("get users: " + data.data);
            if (data.data.length === 0 ) return;

            data.data.forEach(function(u) {
                if (u==userNameInput.value) return;

                var videotag = document.getElementById(u);
                console.log("videotag", videotag);
                if (videotag === null) {
                    CreateVideo(u);
                }
            });

            break;
    }

    return;

}

var delay = 1000;
var timeout = 0;

function CreateVideo(u) {
    var container = document.createElement('div');
    container.id = 'container-'+u;
    container.className = "container";
    var video = document.createElement('video');
    video.id = u;
    video.autoplay = true;
    container.appendChild(video);
    var link = document.createElement('a');
    link.href = '#';
    link.className = 'video-label';
    link.innerHTML = '<b> call to '+u+'</b>';
    var linkClick = false;
    link.onclick = function(e) {
        e.preventDefault();
        linkClick = true;

        if (pcs[u] == null) {
            callTo(u);
            link.innerHTML = '<b> hang up '+u+'</b>';
        } else {
            stopPC(u);
            signalingChannel.send({cmd:"hangup",sender: u});
            link.innerHTML = '<b> call '+u+'</b>';
        }
        return false;
    };
    container.appendChild(link);

    container.onclick = function (e) {
        e.preventDefault();

        if (linkClick) {
            linkClick = false;
            return false;
        }

        if (container.style.width === "100%") {
            container.style.margin = "5%";
            container.style.width = "";
            video.style.width = "";
            video.style.height = "";
        } else {
            container.style.margin = "0%";
            container.style.width = "100%";
            video.style.width = "100%";
            video.style.height = "100%";
        }
        return false;
    };

    usersDiv.appendChild(container);

    if (!!autoCallCheckbox.value) {
        timeout += delay;
        link.innerHTML = '<b>Calling '+u+'...</b>';
        setTimeout(function () {
            callTo(u);
            link.innerHTML = '<b> hang up '+u+'</b>';
            timeout -= delay;
        }, timeout);
    }
}

function stopPC(callee) {
    var pc = pcs[callee];
    if (pc == null) return;

    if (pc.signalingState =="closed") {
        delete pcs[callee];
    } else {
        pc.getRemoteStreams().forEach(function(s) {
            s.getTracks().forEach(function(t) {
                t.stop();
                s.removeTrack(t);
            });
            pc.removeStream(s);
        });

        pc.getLocalStreams().forEach(function(s) {
            s.getTracks().forEach(function(t) {
                t.stop();
                s.removeTrack(t);
            });
            pc.removeStream(s);
        });

        pc.close();
        delete pcs[callee];
    }
}

function logError(error) {
    console.log(error.name + ": " + error.message);
}