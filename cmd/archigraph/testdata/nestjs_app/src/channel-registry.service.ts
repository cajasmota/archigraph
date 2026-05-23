import { Injectable } from '@nestjs/common';

/**
 * ChannelRegistry tracks all active WebSocket channels.
 * Marked @Injectable so NestJS injects it into dependent modules.
 */
@Injectable()
export class ChannelRegistry {
  private readonly channels = new Map<string, Set<string>>();

  register(channelId: string, clientId: string): void {
    if (!this.channels.has(channelId)) {
      this.channels.set(channelId, new Set());
    }
    this.channels.get(channelId)!.add(clientId);
  }

  unregister(channelId: string, clientId: string): void {
    this.channels.get(channelId)?.delete(clientId);
  }

  getClients(channelId: string): string[] {
    return Array.from(this.channels.get(channelId) ?? []);
  }
}
