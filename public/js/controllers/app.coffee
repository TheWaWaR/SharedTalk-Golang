
initTabs = () ->
    $("#room-tabs a").click (e) ->
        e.preventDefault()
        $(this).tab "show"

    $("#room-tabs a:first").tab "show";
    console.log "show first"

    
showTab = (selector) ->
    $tab = $(selector)
    $tab.tab "show"
    ($tab.find ".notifier").addClass "hide" # hide notifier
    console.log "showTab:", selector


Object.size = (obj) ->
    size = 0
    for k, v of obj
        if obj.hasOwnProperty k
            size++
    return size


loadJoined = () ->
    ridsStr = $.cookie("room-ids")
    joinedRooms = {}
    if ridsStr?
        for rid in ridsStr.split(",")
            joinedRooms[rid] = true
    return joinedRooms
    
saveJoined = (joinedRooms) ->
    rids = (rid for rid, v of joinedRooms)
    $.cookie("room-ids", rids)


# Angular things
ws = null
chatApp = angular.module "chatApp", []

chatApp.factory "ChatService", ()->
    service = {}
    
    service.setOnmessage = (callback) ->
        service.onmessage = callback
    service.setOnopen = (callback) ->
        service.onopen = callback
        
    service.connect = () ->
        return if service.ws

        ws = new WebSocket "ws://#{location.hostname}:9091"
        ws.onopen = (event) ->
            service.onopen(event)
        ws.onmessage = (event) ->
            service.onmessage(event)

        service.ws = ws

    return service


chatApp.controller "Ctrl", ['$scope', 'ChatService', ($scope, ChatService) ->
    $scope.templateUrl = "/html/main.html"
    $scope.user = {}            # key : value
    $scope.rooms = {}
    $scope.joinedRooms = {}     # room.id : true|false
    $scope.joinedSize = 0
    $scope.members = {}         # {room.id : {user.id: user}}
    $scope.history = {}         # room.id : [message, ...]
    $scope.users = {}           # user.id : [user, ...]
    $scope.visitors = {}        # visitor.id : [visitor, ...]
    $scope.currentRid = null

    # Function for pages
    $scope.send = (type, oid) ->
        # body = $('#message-input-'+id).val()
        body = this.text
        if body? and body.length > 0
            msg = {path:'message', type:type, oid: (parseInt oid), body: body}
            console.log 'send:', msg
            ws.send (JSON.stringify msg)
            # $('#message-input-'+id).val ""
            this.text = ""

    $scope.showTab = (rid) ->
        console.log "$scope.showTab:", rid
        $scope.currentRid = rid
        showTab "#rtab-#{$scope.currentRid}"
        return null             # Because augular function cann't return DOM object!!!

    $scope.toggleRoom = (rid) ->
        console.log "toggleRoom", rid, $scope.joinedRooms, (rid of $scope.joinedRooms)
        if rid of $scope.joinedRooms
            msg = {'path': 'leave', 'oid': rid}
        else
            msg = {'path': 'join', 'oid': rid}
        ws.send (JSON.stringify msg)

    $scope.objSize = (obj) ->
        return Object.size obj
        

    # Service things
    ChatService.setOnopen () ->
        token = $.cookie('token')
        msg = {}
        if token?
            msg.path = 'online'
            msg.type = 'user'
            msg.token = token
        else
            msg.path = 'create_client'
            msg.type = 'user'
        ws.send (JSON.stringify msg)
        console.log 'Opened'

                
    ChatService.setOnmessage (event) ->
        data = JSON.parse event.data
        console.log '<<DATA>>:', data
        switch data.path
            when 'create_client'
                msg = {path:'online', type:'user'}
                msg.token = data.token
                $.cookie('token', data.token)
                ws.send (JSON.stringify msg)
            when 'online'
                if data.reset?
                    $.removeCookie 'token'
                    $.removeCookie 'room-ids'
                    msg = {path:'create_client', type:'user'}
                    ws.send (JSON.stringify msg)
                    console.log 'Reset'
                else
                    $scope.user.oid = data.oid
                    $scope.user.name = data.name
                    msg = {path:'rooms'}
                    ws.send (JSON.stringify msg)
                    console.log 'Onlined'
                    
            when 'rooms'
                $scope.joinedRooms = do loadJoined
                for room in data.rooms
                    $scope.rooms[room.oid] = room
                    if room.oid of $scope.joinedRooms
                        room.joined = true
                        msg = {'path': 'join', 'oid': room.oid}
                        ws.send (JSON.stringify msg)
                    else
                        room.joined = false
                console.log 'rooms:', $scope.rooms
                # just fill the currentRid
                for room in data.rooms
                    $scope.currentRid = room.oid
                    break
                    
            when 'join'
                msg = {path: 'members', oid: data.oid}
                ws.send (JSON.stringify msg)
                console.log 'Joined:', data
                $scope.rooms[data.oid].joined = true
                $scope.joinedRooms[data.oid] = true
                $scope.joinedSize += 1
                saveJoined $scope.joinedRooms
                
            when 'leave'
                delete $scope.members[data.oid]
                delete $scope.history[data.oid]
                delete $scope.joinedRooms[data.oid]
                $scope.rooms[data.oid].joined = false
                $scope.joinedSize -= 1
                
                # Find next currentRid
                if data.oid == $scope.currentRid
                    for rid, v of $scope.joinedRooms
                        $scope.showTab rid
                        break
                saveJoined $scope.joinedRooms
                
            when 'members'
                $scope.members[data.oid] = {}
                for member in data.members
                    $scope.members[data.oid][member.oid] = member
                console.log 'Get members:', $scope.members, data.members
                msg = {path: 'history', type:'room', oid: data.oid}
                ws.send (JSON.stringify msg)
                
            when 'history'
                $scope.history[data.oid] = data.messages
                console.log 'Get history:', data.oid, data.messages
                $scope.showTab data.oid
                
            when 'presence'
                switch data.to_type
                    when 'room'
                        console.log "OLD::", $scope.members
                        switch data.action
                            when 'join'
                                $scope.members[data.to_id][data.member.oid] = data.member
                                console.log "NEW::", $scope.members
                            when 'leave'
                                delete $scope.members[data.to_id][data.member.oid]
                        
            when 'message'
                switch data.to_type
                    when 'room'
                        console.log 'received message:', data
                        if data.to_id != $scope.currentRid
                            $("#rtab-#{data.to_id} .notifier").removeClass "hide"
                        console.log $("#rtab-#{data.to_id} .notifier"), $scope.currentRid, data.to_id
                        $scope.history[data.to_id].push data
                        # $('#room-'+data.oid).append "#{data.from}: #{data.body}<br />"
                console.log 'Message.type:', data.to_type

        $scope.$apply()
        
    ChatService.connect()
    
    return 'ok'
]
