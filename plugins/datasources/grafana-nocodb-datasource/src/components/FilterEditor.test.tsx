import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';

import { FilterEditor } from './FilterEditor';
import { FilterGroup, emptyRootGroup } from '../filter';
import { FieldInfo } from '../types';

const fields: FieldInfo[] = [
  { title: 'Name', type: 'SingleLineText' },
  { title: 'Age', type: 'Number' },
];

describe('FilterEditor', () => {
  it('adds a filter row when clicking "Add filter"', () => {
    let group: FilterGroup = emptyRootGroup();
    const onChange = (g: FilterGroup) => {
      group = g;
    };
    render(<FilterEditor group={group} fields={fields} onChange={onChange} />);

    fireEvent.click(screen.getByRole('button', { name: /add filter$/i }));
    expect(group.children).toHaveLength(1);
    expect(group.children[0].kind).toBe('condition');
  });

  it('adds a nested group when clicking "Add filter group"', () => {
    let group: FilterGroup = emptyRootGroup();
    const onChange = (g: FilterGroup) => {
      group = g;
    };
    render(<FilterEditor group={group} fields={fields} onChange={onChange} />);

    fireEvent.click(screen.getByRole('button', { name: /add filter group/i }));
    expect(group.children).toHaveLength(1);
    expect(group.children[0].kind).toBe('group');
  });

  it('renders existing condition rows', () => {
    const group: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [{ kind: 'condition', field: 'Age', op: 'gt', value: '30' }],
    };
    render(<FilterEditor group={group} fields={fields} onChange={() => {}} />);
    // The "Where" connector label is shown for the first row.
    expect(screen.getByText('Where')).toBeInTheDocument();
    // The value input reflects the stored value.
    expect(screen.getByDisplayValue('30')).toBeInTheDocument();
  });

  it('shows an editable connector control from the second row onward', () => {
    const group: FilterGroup = {
      kind: 'group',
      connector: 'and',
      children: [
        { kind: 'condition', field: 'Age', op: 'gt', value: '30' },
        { kind: 'condition', field: 'Age', op: 'lt', value: '10' },
      ],
    };
    render(<FilterEditor group={group} fields={fields} onChange={() => {}} />);
    // First row label + a connector selector for the second row.
    expect(screen.getByText('Where')).toBeInTheDocument();
    expect(screen.getByLabelText('Filter connector')).toBeInTheDocument();
  });
});
