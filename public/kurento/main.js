'use strict';

var signalingMethod = "websocket"; // websocket | post

var audioCodec = "opus"
var videoCodec = "vp8"

var firstPage = document.getElementById('first-page');
var roomPage = document.getElementById('room-page');
var joinButton = document.getElementById('joinButton');
//var stopButton = document.getElementById('stopButton');
//var testButton = document.getElementById('testButton');
joinButton.disabled = false;
//stopButton.disabled = true
//testButton.disabled = true

joinButton.onclick = join;
/*
stopButton.onclick = stop;
testButton.onclick = test;
*/

var localVideo = document.getElementById('localVideo');

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


    signalingChannel.start(
        function () {
            signalingChannel.send({
                cmd:"joinRoom",
                room: roomNameInput.value,
                user: userNameInput.value
            });
            firstPage.style.display = "none";
            roomPage.style.display = "block";
            callTo(userNameInput.value);
        }
    );

}

function stop() {
    userNameInput.disabled = false;
    roomNameInput.disabled = false;
    joinButton.disabled = false;
 //   stopButton.disabled = true;
  //  testButton.disabled = true;

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
    });

    firstPage.style.display = "block";
    roomPage.style.display = "none";
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
	this.start = function(cb) {
        this.socket = new WebSocket("wss://"+window.location.host+"/kurento");
        this.socket.onopen = function() {
             cb();
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
    // {
    //     offerToReceiveAudio: {
    //         exact: 1
    //     },
    //     offerToReceiveVideo:
    //         {
    //             exact: 1
    //         },
    //     advanced: [
    //         {
    //             enableDtlsSrtp: {
    //                 exact: true
    //             }
    //         }
    //     ]
    // };

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
            // remoteVideo.pause();
            remoteVideo.srcObject = e.stream;
            // remoteVideo.load();
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
        navigator.mediaDevices.getUserMedia({video: true, audio: true}).then(function (stream) {
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
        return stop();
    }

    if (data.cmd === undefined) return;


    switch(data.cmd) {
        case 'receiveVideoAnswer':
            console.log('answer received');
            var pc = pcs[data.name];
            pc.setRemoteDescription(new RTCSessionDescription({"type":"answer","sdp":data.sdpAnswer}))
                .catch(logError);
            break;

        case 'iceCandidate':
            console.log('iceCandidate');
            var pc = pcs[data.name];
            pc.addIceCandidate(new RTCIceCandidate(data.candidate)).catch(logError);
            break;

        case  'endpointWasConnected':


            /*
            if (data.name==userNameInput.value) return;
            pc = pcs[data.name];
            if (!!pc.restarted) return;
            var videotag = document.getElementById(data.name);
            console.log('endpointWasConnected',"YO!");
            callTo(data.name);

            pc.restarted = true;
            videotag.pause();
            videotag.load();
            */
            break;

        case 'newParticipantArrived':
            if (data.name==userNameInput.value) return;

            var videotag = document.getElementById(data.name);

            console.log("videotag", videotag);
            if (videotag === null) {
                var container = document.createElement('div');
                container.className = "container";
                var video = document.createElement('video');
                video.id = data.name;
                video.autoplay = true;
                container.appendChild(video);
                var link = document.createElement('a');
                link.href = '#';
                link.className = 'video-label';
                link.innerHTML = '<b> call '+data.name+'</b>';
                link.onclick = function() {
                    callTo(data.name);
                    return false;
                };
                container.appendChild(link);

                usersDiv.appendChild(container);
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


            /*
            Object.keys(pcs).map(function(key, ind){
                if (pcs[key].signalingState =="closed") {
                    delete pcs[key];
                } else if (pcs[key].signalingState !="closed" && !data.data.includes(key)) {
                    pcs[key].getLocalStreams().forEach(function(s) {
                        s.getTracks().forEach(function(t) {t.stop()});
                    });
                    pcs[key].close();
                    delete pcs[key];
                }
            });

            Array.from(document.getElementsByClassName("container")).forEach(function(cont){
                if (!data.data.includes(cont.getElementsByTagName("video")[0].id)){
                    cont.parentNode.removeChild(cont);
                }
            });
            */

            break;
    }

    return;

}

function logError(error) {
    console.log(error.name + ": " + error.message);
}