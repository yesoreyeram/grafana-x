import { ScopedVars } from '@grafana/data';

import { DataSource } from './datasource';
import { IntercomQuery } from './types';

const makeDS = () =>
  new DataSource({
    id: 1,
    uid: 'test',
    type: 'yesoreyeram-intercom-datasource',
    name: 'Intercom',
    jsonData: { baseURL: 'https://api.intercom.io', intercomVersion: '2.11' },
  } as any);

jest.mock('@grafana/runtime', () => ({
  ...jest.requireActual('@grafana/runtime'),
  getTemplateSrv: () => ({
    replace: (value: string) => (value === '$role' ? 'user' : value),
  }),
}));

describe('Intercom DataSource', () => {
  it('returns the default query', () => {
    const ds = makeDS();
    expect(ds.getDefaultQuery({} as any)).toMatchObject({ queryType: 'conversations' });
  });

  it('applyTemplateVariables interpolates scalar fields and filter values', () => {
    const ds = makeDS();
    const query: IntercomQuery = {
      refId: 'A',
      queryType: 'contacts',
      role: '$role',
      filters: [{ field: 'role', operator: '=', value: '$role' }],
    };
    const out = ds.applyTemplateVariables(query, {} as ScopedVars);
    expect(out.role).toBe('user');
    expect(out.filters?.[0].value).toBe('user');
  });

  it('getAdmins fetches admins via the resource handler', async () => {
    const ds = makeDS();
    const getResource = jest.fn().mockResolvedValue({
      admins: [{ id: '1', name: 'Alice', email: 'alice@acme.io' }],
    });
    (ds as any).getResource = getResource;

    const admins = await ds.getAdmins();
    expect(getResource).toHaveBeenCalledWith('admins');
    expect(admins).toEqual([{ id: '1', name: 'Alice', email: 'alice@acme.io' }]);
  });

  it('getTeams and getTags fetch via the resource handler', async () => {
    const ds = makeDS();
    const getResource = jest
      .fn()
      .mockResolvedValueOnce({ teams: [{ id: 't1', name: 'Support' }] })
      .mockResolvedValueOnce({ tags: [{ id: 'g1', name: 'vip' }] });
    (ds as any).getResource = getResource;

    await expect(ds.getTeams()).resolves.toEqual([{ id: 't1', name: 'Support' }]);
    await expect(ds.getTags()).resolves.toEqual([{ id: 'g1', name: 'vip' }]);
  });
});
