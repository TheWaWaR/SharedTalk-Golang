
initTabs = () ->
    $("#room-tabs a").click (e) ->
        e.preventDefault()
        $(this).tab "show"

    $("#room-tabs a:first").tab "show";
    console.log "show first"

    
showTab = (selector) ->
    $(selector).addClass "hide"
    
hideTab = (selector) ->
    $(selector).removeClass "hide"


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
    $scope.user = {}
    $scope.rooms = []
    $scope.members = {}
    $scope.history = {}
    $scope.users = {}
    $scope.visitors = {}
    $scope.currentRid = null

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
        $scope.currentRid = rid
        showTab "#rtab-#{$scope.currentRid} .notifier"
        return

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
                $scope.rooms = data.rooms
                for room in data.rooms
                    msg = {path: 'join', oid: room.oid}
                    ws.send (JSON.stringify msg)
                console.log 'rooms:', $scope.rooms
                for room in data.rooms
                    $scope.currentRid = room.oid
                    break
                    
            when 'join'
                msg = {path: 'members', oid: data.oid}
                ws.send (JSON.stringify msg)
                msg = {path: 'history', type:'room', oid: data.oid}
                ws.send (JSON.stringify msg)
                console.log 'Joined:', data
            when 'members'
                $scope.members[data.oid] = {}
                for member in data.members
                    $scope.members[data.oid][member.oid] = member
                console.log 'Get members:', $scope.members, data.members
            when 'history'
                $scope.history[data.oid] = data.messages
                console.log 'Get history:', data.oid, data.messages
                do initTabs
            when 'presence'
                switch data.to_type
                    when 'room'
                        switch data.action
                            when 'join'
                                $scope.members[data.to_id][data.member.oid] = data.member
                            when 'leave'
                                delete $scope.members[data.to_id][data.member.oid]
            when 'message'
                switch data.to_type
                    when 'room'
                        console.log 'received message:', data
                        if data.to_id != $scope.currentRid
                            hideTab "#rtab-#{data.to_id} .notifier"
                        $scope.history[data.to_id].push data
                        # $('#room-'+data.oid).append "#{data.from}: #{data.body}<br />"
                console.log 'Message.type:', data.to_type

        $scope.$apply()
        
    ChatService.connect()
    
    return 'ok'
]
