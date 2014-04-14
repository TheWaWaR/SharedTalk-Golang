// Generated by CoffeeScript 1.7.1
(function() {
  var chatApp, initTabs, loadJoined, saveJoined, showTab, ws;

  initTabs = function() {
    $("#room-tabs a").click(function(e) {
      e.preventDefault();
      return $(this).tab("show");
    });
    return $("#room-tabs a:first").tab("show");
  };

  showTab = function(selector) {
    var $tab;
    $tab = $(selector);
    $tab.tab("show");
    ($tab.find(".notifier")).addClass("hide");
    return console.log("showTab:", selector);
  };

  Object.size = function(obj) {
    var k, size, v;
    size = 0;
    for (k in obj) {
      v = obj[k];
      if (obj.hasOwnProperty(k)) {
        size++;
      }
    }
    return size;
  };

  loadJoined = function() {
    var joinedRooms, rid, ridsStr, _i, _len, _ref;
    ridsStr = $.cookie("room-ids");
    joinedRooms = {};
    if (ridsStr != null) {
      _ref = ridsStr.split(",");
      for (_i = 0, _len = _ref.length; _i < _len; _i++) {
        rid = _ref[_i];
        joinedRooms[rid] = true;
      }
    }
    return joinedRooms;
  };

  saveJoined = function(joinedRooms) {
    var rid, rids, v;
    rids = (function() {
      var _results;
      _results = [];
      for (rid in joinedRooms) {
        v = joinedRooms[rid];
        _results.push(rid);
      }
      return _results;
    })();
    return $.cookie("room-ids", rids);
  };

  ws = null;

  chatApp = angular.module("chatApp", []);

  chatApp.factory("ChatService", function() {
    var service;
    service = {};
    service.setOnmessage = function(callback) {
      return service.onmessage = callback;
    };
    service.setOnopen = function(callback) {
      return service.onopen = callback;
    };
    service.connect = function() {
      if (service.ws) {
        return;
      }
      ws = new WebSocket("ws://" + location.hostname + ":9091");
      ws.onopen = function(event) {
        return service.onopen(event);
      };
      ws.onmessage = function(event) {
        return service.onmessage(event);
      };
      return service.ws = ws;
    };
    return service;
  });

  chatApp.controller("Ctrl", [
    '$scope', 'ChatService', function($scope, ChatService) {
      $scope.templateUrl = "/html/main.html";
      $scope.user = {};
      $scope.rooms = {};
      $scope.joinedRooms = {};
      $scope.joinedSize = 0;
      $scope.members = {};
      $scope.history = {};
      $scope.users = {};
      $scope.visitors = {};
      $scope.currentRid = null;
      $scope.sections = {
        rooms: {
          name: "rooms",
          title: "Rooms",
          placement: "left",
          icon: "fa-group"
        },
        users: {
          name: "users",
          title: "Users",
          placement: "top",
          icon: "fa-user"
        },
        visitors: {
          name: "visitors",
          title: "Visitors",
          placement: "right",
          icon: "fa-male"
        }
      };
      $scope.curSection = "rooms";
      $scope.send = function(type, oid) {
        var body, msg;
        body = this.text;
        if ((body != null) && body.length > 0) {
          msg = {
            path: 'message',
            type: type,
            oid: parseInt(oid),
            body: body
          };
          console.log('send:', msg);
          ws.send(JSON.stringify(msg));
          return this.text = "";
        }
      };
      $scope.changeSection = function(key) {
        if (key in $scope.sections) {
          return $scope.curSection = key;
        } else {
          return console.log("changeSection.errorKey:", key);
        }
      };
      $scope.showTab = function(rid) {
        console.log("$scope.showTab:", rid);
        $scope.currentRid = rid;
        showTab("#rtab-" + $scope.currentRid);
        return null;
      };
      $scope.toggleRoom = function(rid) {
        var msg;
        console.log("toggleRoom", rid, $scope.joinedRooms, rid in $scope.joinedRooms);
        if (rid in $scope.joinedRooms) {
          msg = {
            'path': 'leave',
            'oid': rid
          };
        } else {
          msg = {
            'path': 'join',
            'oid': rid
          };
        }
        return ws.send(JSON.stringify(msg));
      };
      $scope.objSize = function(obj) {
        return Object.size(obj);
      };
      ChatService.setOnopen(function() {
        var msg, token;
        token = $.cookie('token');
        msg = {};
        if (token != null) {
          msg.path = 'online';
          msg.type = 'user';
          msg.token = token;
        } else {
          msg.path = 'create_client';
          msg.type = 'user';
        }
        ws.send(JSON.stringify(msg));
        return console.log('Opened');
      });
      ChatService.setOnmessage(function(event) {
        var containerId, data, member, msg, rid, room, v, _i, _j, _k, _len, _len1, _len2, _ref, _ref1, _ref2, _ref3;
        data = JSON.parse(event.data);
        console.log('<<DATA>>:', data);
        switch (data.path) {
          case 'create_client':
            msg = {
              path: 'online',
              type: 'user'
            };
            msg.token = data.token;
            $.cookie('token', data.token);
            ws.send(JSON.stringify(msg));
            break;
          case 'online':
            if (data.reset != null) {
              $.removeCookie('token');
              $.removeCookie('room-ids');
              msg = {
                path: 'create_client',
                type: 'user'
              };
              ws.send(JSON.stringify(msg));
              console.log('Reset');
            } else {
              $scope.user.oid = data.oid;
              $scope.user.name = data.name;
              msg = {
                path: 'rooms'
              };
              ws.send(JSON.stringify(msg));
              console.log('Onlined');
            }
            break;
          case 'rooms':
            $scope.joinedRooms = loadJoined();
            _ref = data.rooms;
            for (_i = 0, _len = _ref.length; _i < _len; _i++) {
              room = _ref[_i];
              $scope.rooms[room.oid] = room;
              if (room.oid in $scope.joinedRooms) {
                room.joined = true;
                msg = {
                  'path': 'join',
                  'oid': room.oid
                };
                ws.send(JSON.stringify(msg));
              } else {
                room.joined = false;
              }
            }
            console.log('rooms:', $scope.rooms);
            _ref1 = data.rooms;
            for (_j = 0, _len1 = _ref1.length; _j < _len1; _j++) {
              room = _ref1[_j];
              $scope.currentRid = room.oid;
              break;
            }
            initTabs();
            break;
          case 'join':
            msg = {
              path: 'members',
              oid: data.oid
            };
            ws.send(JSON.stringify(msg));
            console.log('Joined:', data);
            $scope.rooms[data.oid].joined = true;
            $scope.joinedRooms[data.oid] = true;
            $scope.joinedSize += 1;
            saveJoined($scope.joinedRooms);
            break;
          case 'leave':
            delete $scope.members[data.oid];
            delete $scope.history[data.oid];
            delete $scope.joinedRooms[data.oid];
            $scope.rooms[data.oid].joined = false;
            $scope.joinedSize -= 1;
            if (data.oid === $scope.currentRid) {
              _ref2 = $scope.joinedRooms;
              for (rid in _ref2) {
                v = _ref2[rid];
                $scope.showTab(rid);
                break;
              }
            }
            saveJoined($scope.joinedRooms);
            break;
          case 'members':
            $scope.members[data.oid] = {};
            _ref3 = data.members;
            for (_k = 0, _len2 = _ref3.length; _k < _len2; _k++) {
              member = _ref3[_k];
              $scope.members[data.oid][member.oid] = member;
            }
            console.log('Get members:', $scope.members, data.members);
            msg = {
              path: 'history',
              type: 'room',
              oid: data.oid
            };
            ws.send(JSON.stringify(msg));
            break;
          case 'history':
            $scope.history[data.oid] = data.messages;
            console.log('Get history:', data.oid, data.messages);
            $scope.showTab(data.oid);
            break;
          case 'presence':
            switch (data.to_type) {
              case 'room':
                console.log("OLD::", $scope.members);
                switch (data.action) {
                  case 'join':
                    $scope.members[data.to_id][data.member.oid] = data.member;
                    console.log("NEW::", $scope.members);
                    break;
                  case 'leave':
                    delete $scope.members[data.to_id][data.member.oid];
                }
            }
            break;
          case 'message':
            switch (data.to_type) {
              case 'room':
                console.log('received message:', data);
                if (data.to_id !== $scope.currentRid) {
                  $("#rtab-" + data.to_id + " .notifier").removeClass("hide");
                }
                console.log($("#rtab-" + data.to_id + " .notifier"), $scope.currentRid, data.to_id);
                $scope.history[data.to_id].push(data);
                containerId = "#room-" + data.to_id + "-messages";
                $(containerId).animate({
                  scrollTop: $(containerId).scrollTop() + $("" + containerId + " .message:last").height()
                }, {
                  duration: 200
                });
                console.log($("" + containerId + " .message:last").offset().top);
            }
            console.log('Message.type:', data.to_type);
        }
        return $scope.$apply();
      });
      ChatService.connect();
      return 'ok';
    }
  ]);

}).call(this);
