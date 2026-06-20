import React from 'react';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';

import { QueryEditor, tableChangePatch } from './QueryEditor';
import { NocoDBQuery } from '../types';

function setup(overrides: Partial<NocoDBQuery> = {}) {
  const query: NocoDBQuery = {
    refId: 'A',
    queryType: 'records',
    tableId: 'm_1',
    ...overrides,
  };
  const onChange = jest.fn();
  const onRunQuery = jest.fn();
  const datasource: any = {
    baseId: '',
    getTables: jest.fn().mockResolvedValue([]),
    getFields: jest.fn().mockResolvedValue([
      { title: 'Name', type: 'SingleLineText' },
      { title: 'Age', type: 'Number' },
    ]),
    getViews: jest.fn().mockResolvedValue([]),
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

describe('tableChangePatch', () => {
  it('clears view/filters/sort/fields when switching to a different table', () => {
    expect(tableChangePatch('m_2', 'p_1', true)).toEqual({
      tableId: 'm_2',
      baseId: 'p_1',
      viewId: '',
      filterTree: '',
      sort: '',
      fields: '',
    });
  });

  it('keeps options when the table is unchanged', () => {
    expect(tableChangePatch('m_1', 'p_1', false)).toEqual({ tableId: 'm_1', baseId: 'p_1' });
  });
});
