import { Injectable } from '@nestjs/common';
import { ChannelRegistry } from './channel-registry.service';

/**
 * NotificationService sends messages to registered clients through the
 * ChannelRegistry. Demonstrates a second @Injectable class so we can spot-
 * check that the fold works for more than one service per fixture.
 */
@Injectable()
export class NotificationService {
  constructor(private readonly registry: ChannelRegistry) {}

  broadcast(channelId: string, message: string): void {
    const clients = this.registry.getClients(channelId);
    clients.forEach(clientId => {
      console.log(`[${channelId}] -> ${clientId}: ${message}`);
    });
  }
}
