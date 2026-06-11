export class SyncService {
  async sync(req: Request) {
    const rows = await this.repo.find();
    if (req.body.force) {
      await this.repo.save({ name: 'seed' });
    }
    for (const row of rows) {
      await fetch('https://api.example.com/notify', { method: 'POST' });
    }
    return { ok: true };
  }
}
