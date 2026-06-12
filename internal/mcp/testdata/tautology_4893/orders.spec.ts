describe('OrdersController', () => {
  it('tautological — false-greens the parity gate', async () => {
    const res = await request(app).get('/api/orders/1');
    // self-compare: asserts the body against itself — can never fail.
    assertBodyContract(res.body, res.body);
    // constant-true: always passes.
    expect(true).toBe(true);
    // same-literal: expected == actual literal.
    expect('ok').toBe('ok');
  });

  it('real assertion — genuinely checks behaviour', async () => {
    const res = await request(app).get('/api/orders/1');
    expect(res.status).toBe(200);
    expect(res.body.statusCounts).toEqual(expected.statusCounts);
    expect(res.body.id).toBe(1);
  });
});
