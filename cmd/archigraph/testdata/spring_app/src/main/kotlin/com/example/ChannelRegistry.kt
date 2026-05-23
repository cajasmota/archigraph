package com.example

import org.springframework.stereotype.Service

/**
 * ChannelRegistry tracks active WebSocket channels.
 * The @Service annotation causes the Kotlin extractor to emit SCOPE.Service
 * (with provenance="@Service"), while the hierarchy extractor simultaneously
 * emits SCOPE.Component/class for the same symbol. Without the #1700 fix,
 * both nodes survive: this class tests that SCOPE.Service absorbs the shadow.
 */
@Service
class ChannelRegistry {
    private val channels = mutableMapOf<String, MutableSet<String>>()

    fun register(channelId: String, clientId: String) {
        channels.getOrPut(channelId) { mutableSetOf() }.add(clientId)
    }

    fun unregister(channelId: String, clientId: String) {
        channels[channelId]?.remove(clientId)
    }

    fun getClients(channelId: String): List<String> =
        channels[channelId]?.toList() ?: emptyList()
}
