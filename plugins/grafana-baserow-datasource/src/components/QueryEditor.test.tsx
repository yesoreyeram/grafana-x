import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

import { QueryEditor, tableChangePatch } from './QueryEditor';
import { BaserowQuery } from '../types';

function setup(overrides: Partial<BaserowQuery> = {}, dsOverrides: Record<string, unknown> = {}) {
  const query: BaserowQuery = {
    refId: 'A',
    queryType: 'records',
    tableId: '1',
    ...overrides,
  };
  const onChange = jest.fn();
  const onRunQuery = jest.fn();
  const datasource: any = {
    databaseId: '1',
    authMode: 'token',
    getTables: jest.fn().mockResolvedValue([]),
    getFields: jest.fn().mockResolvedValue([
      { title: 'Name', type: 'text' },
      { title: 'Age', type: 'number' },
    ]),
    getViews: jest.fn().mockResolvedValue([]),
    getDatabases: jest.fn().mockResolvedValue([{ id: '7', title: 'Sales', workspaceName: 'Acme' }]),
    ...dsOverrides,
  };
  render(<QueryEditor query={query} onChange={onChange} onRunQuery={onRunQuery} datasource={datasource} />);
  return { onChange, onRunQuery, datasource };
}

describe('QueryEditor sort', () => {
  it('adds a sort row when clicking "Add sort"', async () => {
    setup();

    expect(screen.queryByText('Asc')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /add sort/i }));

    // A new row renders its direction toggle (Asc/Desc) immediately.
    await waitFor(() => {
      expect(screen.getByText('Asc')).toBeInTheDocument();
      expect(screen.getByText('Desc')).toBeInTheDocument();
    });
  });

  it('renders existing sort rows from the query', () => {
    setup({ sort: '-Age' });
    expect(screen.getByText('Asc')).toBeInTheDocument();
    expect(screen.getByText('Desc')).toBeInTheDocument();
  });
});

describe('QueryEditor filters section', () => {
  it('renders the Filters label and an Add filter action', () => {
    setup();
    expect(screen.getByText('Filters')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add filter$/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /add filter group/i })).toBeInTheDocument();
  });
});

describe('QueryEditor database picker', () => {
  it('shows a database picker in password mode with no fixed database id', async () => {
    const { datasource } = setup({}, { authMode: 'password', databaseId: '' });
    expect(screen.getByText('Select database')).toBeInTheDocument();
    await waitFor(() => {
      expect(datasource.getDatabases).toHaveBeenCalled();
    });
  });

  it('does not show a database picker in token mode', () => {
    const { datasource } = setup({}, { authMode: 'token', databaseId: '1' });
    expect(screen.queryByText('Select database')).not.toBeInTheDocument();
    expect(datasource.getDatabases).not.toHaveBeenCalled();
  });

  it('does not show a database picker in password mode when a database id is configured', () => {
    setup({}, { authMode: 'password', databaseId: '5' });
    expect(screen.queryByText('Select database')).not.toBeInTheDocument();
  });
});

describe('tableChangePatch', () => {
  it('clears view/filters/sort/fields when switching to a different table', () => {
    expect(tableChangePatch('2', true)).toEqual({
      tableId: '2',
      viewId: '',
      filterTree: '',
      sort: '',
      fields: '',
    });
  });

  it('keeps options when the table is unchanged', () => {
    expect(tableChangePatch('1', false)).toEqual({ tableId: '1' });
  });
});
